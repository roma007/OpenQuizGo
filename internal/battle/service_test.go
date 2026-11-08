package battle

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/redis/go-redis/v9"

	"github.com/roma007/OpenQuizGo/migrations"
)

func TestBattleWorkflow(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	prepareDB(t, dbURL)

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	accountID, childID := seedFamily(t, pool)
	questionIDs := seedQuestions(t, pool, 5)
	svc := New(pool, nil)

	state, err := svc.Create(ctx, accountID, childID, 5, "same_device")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if state.Status != "child_turn" || state.QuestionCount != 5 || len(state.Questions) != 5 {
		t.Fatalf("unexpected create state: %+v", state)
	}
	if len(state.Answers) != 0 || state.Result != nil {
		t.Fatal("in-progress battle leaked answers or result")
	}
	if len(state.Questions[0].StemMedia) == 0 {
		t.Fatal("battle question missing stem media")
	}
	if state.JoinCodeExpiresAt != nil {
		t.Fatal("same-device battle should not expose join code expiry")
	}

	childAnswers := make([]Submission, 0, len(questionIDs))
	for _, id := range questionIDs {
		childAnswers = append(childAnswers, Submission{QuestionID: id, Answer: json.RawMessage(`["a"]`)})
	}
	state, err = svc.Submit(ctx, childID, state.ID, "child", childAnswers)
	if err != nil {
		t.Fatalf("child submit: %v", err)
	}
	if state.Status != "parent_turn" || state.ChildScore != 5 || len(state.Answers) != 0 {
		t.Fatalf("unexpected child state: %+v", state)
	}

	if _, err := svc.Submit(ctx, childID, state.ID, "child", childAnswers); err == nil || !isCode(err, "BATTLE_TURN_INVALID") {
		t.Fatalf("duplicate child submit = %v, want BATTLE_TURN_INVALID", err)
	}
	if _, err := svc.Submit(ctx, childID, state.ID, "parent", childAnswers[:2]); err == nil || !isCode(err, "VALIDATION_ERROR") {
		t.Fatalf("short answers = %v, want VALIDATION_ERROR", err)
	}

	parentAnswers := make([]Submission, 0, len(questionIDs))
	for i, id := range questionIDs {
		answer := json.RawMessage(`["a"]`)
		if i%2 == 1 {
			answer = json.RawMessage(`["b"]`)
		}
		parentAnswers = append(parentAnswers, Submission{QuestionID: id, Answer: answer})
	}
	state, err = svc.Submit(ctx, childID, state.ID, "parent", parentAnswers)
	if err != nil {
		t.Fatalf("parent submit: %v", err)
	}
	if state.Status != "finished" || state.ChildScore != 5 || state.ParentScore != 3 {
		t.Fatalf("unexpected finished state: %+v", state)
	}
	if state.Result == nil || state.Result.Winner != "child" {
		t.Fatalf("unexpected result: %+v", state.Result)
	}
	if len(state.Answers) != 10 {
		t.Fatalf("answers = %d, want 10", len(state.Answers))
	}

	loaded, err := svc.Get(ctx, childID, state.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if loaded.Result == nil || loaded.Result.Winner != "child" || len(loaded.Answers) != 10 {
		t.Fatalf("unexpected loaded state: %+v", loaded)
	}
	if _, err := svc.Submit(ctx, childID, state.ID, "parent", parentAnswers); err == nil || !isCode(err, "BATTLE_TURN_INVALID") {
		t.Fatalf("submit after finish = %v, want BATTLE_TURN_INVALID", err)
	}

	// 非本家庭账号不能创建对局。
	var outsider string
	if err := pool.QueryRow(ctx, `
		insert into parent_account (phone_cipher, phone_hash) values ('cipher', 'outsider')
		returning id`).Scan(&outsider); err != nil {
		t.Fatalf("outsider: %v", err)
	}
	if _, err := svc.Create(ctx, outsider, childID, 3, "same_device"); err == nil || !isCode(err, "CHILD_NOT_FOUND") {
		t.Fatalf("outsider create = %v, want CHILD_NOT_FOUND", err)
	}

	// 题库不足时拒绝创建。
	if _, err := pool.Exec(ctx, `delete from family_battle`); err != nil {
		t.Fatalf("clear battles: %v", err)
	}
	if _, err := pool.Exec(ctx, `delete from question`); err != nil {
		t.Fatalf("clear questions: %v", err)
	}
	if _, err := svc.Create(ctx, accountID, childID, 5, "same_device"); err == nil || !isCode(err, "QUESTION_POOL_EMPTY") {
		t.Fatalf("empty pool create = %v, want QUESTION_POOL_EMPTY", err)
	}
}

func isCode(err error, code string) bool {
	var e apiError
	return errors.As(err, &e) && e.Code == code
}

func prepareDB(t *testing.T, url string) {
	t.Helper()
	db, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = db.Close() }()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("dialect: %v", err)
	}
	if err := goose.Up(db, "."); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := db.Exec(`truncate table
		family_battle, family_battle_answer, question,
		child_profile, family_member, family, parent_account
		cascade`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

func seedFamily(t *testing.T, pool *pgxpool.Pool) (accountID, childID string) {
	t.Helper()
	ctx := context.Background()
	var familyID string
	if err := pool.QueryRow(ctx, `
		insert into parent_account (phone_cipher, phone_hash) values ('cipher', 'battle-owner')
		returning id`).Scan(&accountID); err != nil {
		t.Fatalf("account: %v", err)
	}
	if err := pool.QueryRow(ctx, `insert into family (owner_id) values ($1) returning id`,
		accountID).Scan(&familyID); err != nil {
		t.Fatalf("family: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		insert into family_member (family_id, account_id, role, second_factor_verified)
		values ($1, $2, 'owner', true)`, familyID, accountID); err != nil {
		t.Fatalf("member: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		insert into child_profile (family_id, nickname, stage_code, grade_tag)
		values ($1, '对战孩子', 'low', 'g2') returning id`, familyID).Scan(&childID); err != nil {
		t.Fatalf("child: %v", err)
	}
	return accountID, childID
}

func seedQuestions(t *testing.T, pool *pgxpool.Pool, n int) []string {
	t.Helper()
	ids := make([]string, 0, n)
	for i := 0; i < n; i++ {
		var id string
		err := pool.QueryRow(context.Background(), `
			insert into question (
				subject_code, category_code, stage_code, difficulty, type_code, stem,
				stem_media, options, answer, explain, status, version, schema_ver)
			values ('math', 'core', 'low', 2, 'choice', $1,
			        '{"url":"/v1/media/stem.png"}'::jsonb,
			        '[{"id":"a","text":"1","images":["/v1/media/a.png"],"audio":"/v1/media/a.mp3"},{"id":"b","text":"2"}]'::jsonb,
			        '["a"]'::jsonb, '解析', 'online', 1, 1)
			returning id`, "对战题").Scan(&id)
		if err != nil {
			t.Fatalf("question: %v", err)
		}
		ids = append(ids, id)
	}
	return ids
}

func TestRoomBattleWorkflow(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	prepareDB(t, dbURL)

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	accountID, childID := seedFamily(t, pool)
	questionIDs := seedQuestions(t, pool, 5)
	svc := New(pool, nil)

	room, err := svc.Create(ctx, accountID, childID, 5, "room")
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	if room.Status != "waiting" || room.Mode != "room" || len(room.JoinCode) != 6 {
		t.Fatalf("unexpected room: %+v", room)
	}
	if room.JoinCodeExpiresAt == nil {
		t.Fatal("room should expose join code expiry")
	}
	remaining := time.Until(*room.JoinCodeExpiresAt)
	if remaining < 9*time.Minute || remaining > 10*time.Minute+time.Minute {
		t.Fatalf("join code ttl = %v, want about 10m", remaining)
	}

	if _, err := svc.Join(ctx, accountID, childID, "ZZZZZZ"); err == nil || !isCode(err, "BATTLE_NOT_FOUND") {
		t.Fatalf("bad code join = %v, want BATTLE_NOT_FOUND", err)
	}
	joined, err := svc.Join(ctx, accountID, childID, room.JoinCode)
	if err != nil {
		t.Fatalf("join: %v", err)
	}
	if joined.Status != "child_turn" || joined.JoinCode != room.JoinCode {
		t.Fatalf("unexpected joined: %+v", joined)
	}
	if _, err := svc.Join(ctx, accountID, childID, room.JoinCode); err != nil {
		t.Fatalf("idempotent join: %v", err)
	}

	childAnswers := make([]Submission, 0, len(questionIDs))
	for _, id := range questionIDs {
		childAnswers = append(childAnswers, Submission{QuestionID: id, Answer: json.RawMessage(`["a"]`)})
	}
	afterChild, err := svc.Submit(ctx, childID, room.ID, "child", childAnswers)
	if err != nil {
		t.Fatalf("child submit: %v", err)
	}
	if afterChild.Status != "parent_turn" || afterChild.ChildScore != 5 {
		t.Fatalf("unexpected after child: %+v", afterChild)
	}
	parentAnswers := make([]Submission, 0, len(questionIDs))
	for _, id := range questionIDs {
		parentAnswers = append(parentAnswers, Submission{QuestionID: id, Answer: json.RawMessage(`["b"]`)})
	}
	finished, err := svc.Submit(ctx, childID, room.ID, "parent", parentAnswers)
	if err != nil {
		t.Fatalf("parent submit: %v", err)
	}
	if finished.Status != "finished" || finished.Result == nil || finished.Result.Winner != "child" {
		t.Fatalf("unexpected finished: %+v", finished)
	}
	if finished.JoinCode != "" {
		t.Fatal("finished room should not expose join code")
	}
	if finished.JoinCodeExpiresAt != nil {
		t.Fatal("finished room should not expose join code expiry")
	}

	history, err := svc.History(ctx, accountID, childID, 20)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(history) != 1 || history[0].Mode != "room" ||
		history[0].Status != "finished" || history[0].ChildScore != 5 ||
		history[0].ParentScore != 0 || history[0].QuestionCount != 5 {
		t.Fatalf("unexpected history: %+v", history)
	}

	// 过期房间仍保持 waiting，但不能再加入。
	expired, err := svc.Create(ctx, accountID, childID, 5, "room")
	if err != nil {
		t.Fatalf("create expiring room: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		update family_battle set join_code_expires_at = now() - interval '1 minute'
		where id = $1`, expired.ID); err != nil {
		t.Fatalf("expire room: %v", err)
	}
	if _, err := svc.Join(ctx, accountID, childID, expired.JoinCode); err == nil || !isCode(err, "ROOM_EXPIRED") {
		t.Fatalf("expired join = %v, want ROOM_EXPIRED", err)
	}
	expiredState, err := svc.Get(ctx, childID, expired.ID)
	if err != nil {
		t.Fatalf("get expired room: %v", err)
	}
	if expiredState.Status != "waiting" {
		t.Fatalf("expired room status = %q, want waiting", expiredState.Status)
	}
}

func TestDeviceRoomJoin(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	prepareDB(t, dbURL)

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	accountID, childID := seedFamily(t, pool)
	questionIDs := seedQuestions(t, pool, 5)
	svc := New(pool, nil)
	room, err := svc.Create(ctx, accountID, childID, 5, "room")
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	if _, err := svc.JoinAsChild(ctx, childID, "ZZZZZZ"); err == nil || !isCode(err, "BATTLE_NOT_FOUND") {
		t.Fatalf("bad code = %v, want BATTLE_NOT_FOUND", err)
	}
	joined, err := svc.JoinAsChild(ctx, childID, room.JoinCode)
	if err != nil {
		t.Fatalf("device join: %v", err)
	}
	if joined.Status != "child_turn" || joined.ID != room.ID {
		t.Fatalf("unexpected joined: %+v", joined)
	}
	answers := make([]Submission, 0, len(questionIDs))
	for _, id := range questionIDs {
		answers = append(answers, Submission{QuestionID: id, Answer: json.RawMessage(`["a"]`)})
	}
	state, err := svc.Submit(ctx, childID, room.ID, "child", answers)
	if err != nil || state.Status != "parent_turn" || state.ChildScore != 5 {
		t.Fatalf("device child submit = %+v err=%v", state, err)
	}

	expired, err := svc.Create(ctx, accountID, childID, 5, "room")
	if err != nil {
		t.Fatalf("create expired room: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		update family_battle set join_code_expires_at = now() - interval '1 minute'
		where id = $1`, expired.ID); err != nil {
		t.Fatalf("expire: %v", err)
	}
	if _, err := svc.JoinAsChild(ctx, childID, expired.JoinCode); err == nil || !isCode(err, "ROOM_EXPIRED") {
		t.Fatalf("expired join = %v, want ROOM_EXPIRED", err)
	}
}

func TestFamilyRanking(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	prepareDB(t, dbURL)

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	accountID, childA := seedFamily(t, pool)
	var familyID, childB string
	if err := pool.QueryRow(ctx, `select family_id from child_profile where id = $1`, childA).
		Scan(&familyID); err != nil {
		t.Fatalf("family: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		insert into child_profile (family_id, nickname, stage_code)
		values ($1, '第二名', 'low') returning id`, familyID).Scan(&childB); err != nil {
		t.Fatalf("child B: %v", err)
	}

	insert := func(childID string, childScore, parentScore int, minutesAgo int) {
		t.Helper()
		if _, err := pool.Exec(ctx, `
			insert into family_battle (
				family_id, child_id, status, question_ids, child_score, parent_score, finished_at)
			values ($1, $2, 'finished', '[]'::jsonb, $3, $4,
			        now() - ($5::int * interval '1 minute'))`,
			familyID, childID, childScore, parentScore, minutesAgo); err != nil {
			t.Fatalf("insert battle: %v", err)
		}
	}
	// A：胜、胜、负（最新）→ 6 分，当前连胜 0，最佳 2。
	insert(childA, 3, 1, 30)
	insert(childA, 2, 0, 20)
	insert(childA, 1, 2, 10)
	// B：胜、平（最新）→ 4 分，当前连胜 0，最佳 1。
	insert(childB, 3, 1, 25)
	insert(childB, 2, 2, 5)
	// 40 天前的旧对局只应出现在总榜。
	insert(childA, 2, 0, 40*24*60)

	svc := New(pool, nil)
	items, err := svc.Ranking(ctx, accountID, "all")
	if err != nil {
		t.Fatalf("ranking: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("ranking len = %d, want 2", len(items))
	}
	first := items[0]
	if first.ChildID != childA || first.Battles != 4 || first.Wins != 3 ||
		first.Draws != 0 || first.Losses != 1 || first.Points != 9 ||
		first.CurrentStreak != 0 || first.BestStreak != 3 {
		t.Fatalf("unexpected first: %+v", first)
	}
	if !hasMedal(first, "first_win") || !hasMedal(first, "streak_3") ||
		!hasMedal(first, "champion") || hasMedal(first, "veteran") ||
		hasMedal(first, "all_win") {
		t.Fatalf("unexpected first medals: %+v", first.Medals)
	}
	second := items[1]
	if second.ChildID != childB || second.Battles != 2 || second.Wins != 1 ||
		second.Draws != 1 || second.Points != 4 || second.BestStreak != 1 {
		t.Fatalf("unexpected second: %+v", second)
	}
	if !hasMedal(second, "first_win") || hasMedal(second, "champion") {
		t.Fatalf("unexpected second medals: %+v", second.Medals)
	}

	week, err := svc.Ranking(ctx, accountID, "week")
	if err != nil {
		t.Fatalf("week ranking: %v", err)
	}
	if len(week) != 2 || week[0].ChildID != childA || week[0].Battles != 3 ||
		week[0].Points != 6 {
		t.Fatalf("unexpected week ranking: %+v", week)
	}
	month, err := svc.Ranking(ctx, accountID, "month")
	if err != nil {
		t.Fatalf("month ranking: %v", err)
	}
	if month[0].Battles != 3 || month[0].Points != 6 {
		t.Fatalf("unexpected month ranking: %+v", month)
	}
}

func TestEventBroker(t *testing.T) {
	broker := newEventBroker(nil)
	ch, unsubscribe := broker.subscribe(context.Background(), "battle-1")
	defer unsubscribe()

	broker.publish(context.Background(), "other-battle")
	select {
	case <-ch:
		t.Fatal("received event for another battle")
	default:
	}

	broker.publish(context.Background(), "battle-1")
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("did not receive published event")
	}

	unsubscribe()
	broker.publish(context.Background(), "battle-1")
	select {
	case <-ch:
		t.Fatal("received event after unsubscribe")
	default:
	}
}

func TestEventBrokerSharedSubscribers(t *testing.T) {
	ctx := context.Background()
	broker := newEventBroker(nil)
	ch1, unsubscribe1 := broker.subscribe(ctx, "battle-shared")
	ch2, unsubscribe2 := broker.subscribe(ctx, "battle-shared")

	broker.publish(ctx, "battle-shared")
	waitEvent(t, ch1, "first subscriber")
	waitEvent(t, ch2, "second subscriber")

	unsubscribe1()
	broker.publish(ctx, "battle-shared")
	waitEvent(t, ch2, "remaining subscriber")
	select {
	case <-ch1:
		t.Fatal("unsubscribed subscriber received event")
	case <-time.After(100 * time.Millisecond):
	}

	unsubscribe2()
	broker.mu.Lock()
	sharedCount := len(broker.shared)
	localCount := len(broker.subs)
	broker.mu.Unlock()
	if sharedCount != 0 || localCount != 0 {
		t.Fatalf("shared subscriptions = %d, local subscriptions = %d, want 0/0",
			sharedCount, localCount)
	}

	broker.publish(ctx, "battle-shared")
	select {
	case <-ch2:
		t.Fatal("subscriber received event after unsubscribe")
	case <-time.After(100 * time.Millisecond):
	}
}

func waitEvent(t *testing.T, ch <-chan int64, name string) int64 {
	t.Helper()
	select {
	case version := <-ch:
		return version
	case <-time.After(time.Second):
		t.Fatalf("%s did not receive published event", name)
		return 0
	}
}

func hasMedal(entry RankingEntry, medal string) bool {
	for _, item := range entry.Medals {
		if item == medal {
			return true
		}
	}
	return false
}

func TestRedisEventBroker(t *testing.T) {
	redisURL := os.Getenv("TEST_REDIS_URL")
	if redisURL == "" {
		t.Skip("TEST_REDIS_URL not set")
	}
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatalf("redis url: %v", err)
	}
	rdb := redis.NewClient(opts)
	defer func() { _ = rdb.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	battleID := fmt.Sprintf("redis-test-%d", time.Now().UnixNano())
	publisher := newEventBroker(rdb)
	subscriber := newEventBroker(rdb)
	ch, unsubscribe := subscriber.subscribe(ctx, battleID)
	defer unsubscribe()

	publisher.publish(ctx, battleID)
	select {
	case <-ch:
	case <-ctx.Done():
		t.Fatal("did not receive Redis event")
	}
}

func TestEventVersionLocal(t *testing.T) {
	ctx := context.Background()
	broker := newEventBroker(nil)
	ch, unsubscribe := broker.subscribe(ctx, "battle-version-local")
	defer unsubscribe()

	var last int64
	for i := 0; i < 5; i++ {
		published := broker.publish(ctx, "battle-version-local")
		if published <= last {
			t.Fatalf("published version %d is not greater than %d", published, last)
		}
		received := waitEvent(t, ch, "local version subscriber")
		if received != published {
			t.Fatalf("received version %d, want %d", received, published)
		}
		last = received
	}
}

func TestEventVersionRedis(t *testing.T) {
	redisURL := os.Getenv("TEST_REDIS_URL")
	if redisURL == "" {
		t.Skip("TEST_REDIS_URL not set")
	}
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatalf("redis url: %v", err)
	}
	rdb := redis.NewClient(opts)
	t.Cleanup(func() { _ = rdb.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	battleID := fmt.Sprintf("redis-version-test-%d", time.Now().UnixNano())
	t.Cleanup(func() { _ = rdb.Del(context.Background(), battleVersionKey(battleID)).Err() })

	publisherA := newEventBroker(rdb)
	publisherB := newEventBroker(rdb)
	subscriberA := newEventBroker(rdb)
	subscriberB := newEventBroker(rdb)
	chA, unsubscribeA := subscriberA.subscribe(ctx, battleID)
	defer unsubscribeA()
	chB, unsubscribeB := subscriberB.subscribe(ctx, battleID)
	defer unsubscribeB()

	first := publisherA.publish(ctx, battleID)
	gotA := waitEvent(t, chA, "redis subscriber A")
	gotB := waitEvent(t, chB, "redis subscriber B")
	if gotA != first || gotB != first {
		t.Fatalf("subscribers saw %d/%d, want %d", gotA, gotB, first)
	}

	second := publisherB.publish(ctx, battleID)
	if second != first+1 {
		t.Fatalf("second version = %d, want %d", second, first+1)
	}
	gotA = waitEvent(t, chA, "redis subscriber A")
	gotB = waitEvent(t, chB, "redis subscriber B")
	if gotA != second || gotB != second {
		t.Fatalf("subscribers saw %d/%d, want %d", gotA, gotB, second)
	}

	if baseline := subscriberA.currentVersion(ctx, battleID); baseline != second {
		t.Fatalf("baseline version = %d, want %d", baseline, second)
	}
}

func TestEventVersionRedisFallback(t *testing.T) {
	// 指向不可达端口，模拟 Redis 不可用，订阅与发布都应回退到本地。
	rdb := redis.NewClient(&redis.Options{
		Addr:         "127.0.0.1:1",
		DialTimeout:  50 * time.Millisecond,
		ReadTimeout:  50 * time.Millisecond,
		WriteTimeout: 50 * time.Millisecond,
		PoolTimeout:  50 * time.Millisecond,
		MaxRetries:   -1,
	})
	defer func() { _ = rdb.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	broker := newEventBroker(rdb)
	ch, unsubscribe := broker.subscribe(ctx, "battle-version-fallback")
	defer unsubscribe()

	var last int64
	for i := 0; i < 3; i++ {
		published := broker.publish(ctx, "battle-version-fallback")
		if published <= last {
			t.Fatalf("fallback version %d is not greater than %d", published, last)
		}
		received := waitEvent(t, ch, "fallback subscriber")
		if received != published {
			t.Fatalf("received version %d, want %d", received, published)
		}
		last = received
	}
}

func TestWriteBattleEventFormat(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeBattleEvent(recorder, recorder, 12)
	want := "id: 12\nevent: update\ndata: {\"version\":12}\n\n"
	if got := recorder.Body.String(); got != want {
		t.Fatalf("event stream = %q, want %q", got, want)
	}
}

func TestSharedRedisEventBroker(t *testing.T) {
	redisURL := os.Getenv("TEST_REDIS_URL")
	if redisURL == "" {
		t.Skip("TEST_REDIS_URL not set")
	}
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatalf("redis url: %v", err)
	}
	rdb := redis.NewClient(opts)
	defer func() { _ = rdb.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	battleID := fmt.Sprintf("redis-shared-test-%d", time.Now().UnixNano())
	broker := newEventBroker(rdb)
	ch1, unsubscribe1 := broker.subscribe(ctx, battleID)
	defer unsubscribe1()
	ch2, unsubscribe2 := broker.subscribe(ctx, battleID)
	defer unsubscribe2()

	broker.mu.Lock()
	shared := broker.shared[battleID]
	sharedCount := len(broker.shared)
	subscriberCount := 0
	if shared != nil {
		subscriberCount = len(shared.subs)
	}
	broker.mu.Unlock()
	if sharedCount != 1 || subscriberCount != 2 {
		t.Fatalf("shared subscriptions = %d with %d subscribers, want 1/2",
			sharedCount, subscriberCount)
	}

	broker.publish(ctx, battleID)
	waitEvent(t, ch1, "first Redis subscriber")
	waitEvent(t, ch2, "second Redis subscriber")

	unsubscribe1()
	broker.publish(ctx, battleID)
	waitEvent(t, ch2, "remaining Redis subscriber")
	select {
	case <-ch1:
		t.Fatal("unsubscribed Redis subscriber received event")
	case <-time.After(100 * time.Millisecond):
	}

	unsubscribe2()
	broker.mu.Lock()
	sharedCount = len(broker.shared)
	broker.mu.Unlock()
	if sharedCount != 0 {
		t.Fatalf("shared subscriptions = %d, want 0 after last unsubscribe", sharedCount)
	}
}
