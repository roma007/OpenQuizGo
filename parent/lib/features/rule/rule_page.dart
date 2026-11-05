import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/api_client.dart';
import '../../core/api/models.dart';
import '../../core/layout/adaptive.dart';
import '../../core/providers.dart';
import '../../core/storage/pending_rule_store.dart';
import '../../data/rule_repository.dart';

class RulePage extends ConsumerStatefulWidget {
  const RulePage({super.key, required this.child});

  final Child child;

  @override
  ConsumerState<RulePage> createState() => _RulePageState();
}

class _RulePageState extends ConsumerState<RulePage> {
  @override
  Widget build(BuildContext context) {
    final rule = ref.watch(ruleProvider(widget.child.id));
    return Scaffold(
      appBar: AppBar(title: Text('${widget.child.nickname} 的观看规则')),
      body: rule.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (error, _) => Center(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Text('加载规则失败：$error'),
              const SizedBox(height: 12),
              OutlinedButton(
                onPressed: () => ref.invalidate(ruleProvider(widget.child.id)),
                child: const Text('重试'),
              ),
            ],
          ),
        ),
        data: (cached) => ContentFrame(
          maxWidth: 900,
          child: _RuleForm(
            childId: widget.child.id,
            initial: cached.data.rule,
            version: cached.data.version,
            staleAt: cached.stale ? cached.cachedAt : null,
          ),
        ),
      ),
    );
  }
}

class _RuleForm extends ConsumerStatefulWidget {
  const _RuleForm({
    required this.childId,
    required this.initial,
    required this.version,
    this.staleAt,
  });

  final String childId;
  final Rule initial;
  final int version;
  final DateTime? staleAt;

  @override
  ConsumerState<_RuleForm> createState() => _RuleFormState();
}

class _RuleFormState extends ConsumerState<_RuleForm> {
  late final TextEditingController _unlock;
  late final TextEditingController _threshold;
  late final TextEditingController _daily;
  late final TextEditingController _offline;
  late final TextEditingController _funCap;
  late final TextEditingController _allowedApps;
  late final Map<String, TextEditingController> _weights;

  late String _preset;
  late String _scope;
  late bool _funEnabled;
  late bool _windowEnabled;
  late TimeOfDay _windowStart;
  late TimeOfDay _windowEnd;

  RulePreview? _preview;
  String? _previewError;
  bool _busy = false;
  bool _syncing = false;
  PendingRule? _pending;

  @override
  void initState() {
    super.initState();
    final rule = widget.initial;
    _unlock = _numController(rule.unlockMinutes);
    _threshold = _numController(rule.passThreshold);
    _daily = _numController(rule.dailyLimit);
    _offline = _numController(rule.offlineFallbackMin);
    _funCap = _numController(rule.funRatioCap);
    _allowedApps = TextEditingController(text: rule.allowedApps.join(', '));
    _preset = rule.weightPreset;
    _scope = rule.scopeMode;
    _funEnabled = rule.funEnabled;
    _windowEnabled = rule.timeWindow.isNotEmpty;
    final window = rule.timeWindow.isNotEmpty ? rule.timeWindow.first : null;
    _windowStart =
        _parseTime(window?.start) ?? const TimeOfDay(hour: 7, minute: 0);
    _windowEnd =
        _parseTime(window?.end) ?? const TimeOfDay(hour: 21, minute: 0);
    _weights = {
      for (final entry in subjectLabels.entries)
        entry.key: _numController(rule.subjectWeight[entry.key] ?? 0),
    };
    _loadPending();
  }

  Future<void> _loadPending() async {
    final pending = await ref
        .read(pendingRuleStoreProvider)
        .read(widget.childId);
    if (!mounted || pending == null) return;
    setState(() => _pending = pending);
    _applyRuleValues(pending.rule);
  }

  void _applyRuleValues(Rule rule) {
    setState(() {
      _unlock.text = _formatNum(rule.unlockMinutes);
      _threshold.text = _formatNum(rule.passThreshold);
      _daily.text = _formatNum(rule.dailyLimit);
      _offline.text = _formatNum(rule.offlineFallbackMin);
      _funCap.text = _formatNum(rule.funRatioCap);
      _allowedApps.text = rule.allowedApps.join(', ');
      _preset = rule.weightPreset;
      _scope = rule.scopeMode;
      _funEnabled = rule.funEnabled;
      _windowEnabled = rule.timeWindow.isNotEmpty;
      final window = rule.timeWindow.isNotEmpty ? rule.timeWindow.first : null;
      _windowStart =
          _parseTime(window?.start) ?? const TimeOfDay(hour: 7, minute: 0);
      _windowEnd =
          _parseTime(window?.end) ?? const TimeOfDay(hour: 21, minute: 0);
      for (final entry in _weights.entries) {
        entry.value.text = _formatNum(rule.subjectWeight[entry.key] ?? 0);
      }
    });
  }

  TextEditingController _numController(num value) =>
      TextEditingController(text: _formatNum(value));

  String _formatNum(num value) {
    if (value is double && value == value.roundToDouble()) {
      return value.toInt().toString();
    }
    return value.toString();
  }

  @override
  void dispose() {
    for (final controller in [
      _unlock,
      _threshold,
      _daily,
      _offline,
      _funCap,
      _allowedApps,
      ..._weights.values,
    ]) {
      controller.dispose();
    }
    super.dispose();
  }

  int _int(TextEditingController controller, int fallback) =>
      int.tryParse(controller.text.trim()) ?? fallback;

  double _double(TextEditingController controller, double fallback) =>
      double.tryParse(controller.text.trim()) ?? fallback;

  Rule _buildRule() {
    final weights = <String, double>{
      for (final entry in _weights.entries) entry.key: _double(entry.value, 0),
    };
    return Rule(
      unlockMinutes: _int(_unlock, 20),
      passThreshold: _double(_threshold, 3),
      subjectWeight: weights,
      funRatioCap: _int(_funCap, 2),
      funEnabled: _funEnabled,
      weightPreset: _preset,
      timeWindow: _windowEnabled
          ? [
              TimeWindow(
                start: _formatTime(_windowStart),
                end: _formatTime(_windowEnd),
              ),
            ]
          : const [],
      dailyLimit: _int(_daily, 60),
      offlineFallbackMin: _int(_offline, 10),
      scopeMode: _scope,
      allowedApps: _scope == 'custom'
          ? _allowedApps.text
                .split(',')
                .map((e) => e.trim())
                .where((e) => e.isNotEmpty)
                .toList()
          : const [],
      timezone: widget.initial.timezone,
    );
  }

  void _applyPreset(String? preset) {
    if (preset == null) return;
    setState(() {
      _preset = preset;
      final presetWeights = weightPresets[preset];
      if (presetWeights != null) {
        presetWeights.forEach((subject, weight) {
          _weights[subject]?.text = _formatNum(weight);
        });
      }
    });
  }

  Future<void> _previewRule() async {
    setState(() {
      _busy = true;
      _previewError = null;
    });
    try {
      final preview = await ref
          .read(ruleRepositoryProvider)
          .preview(widget.childId, _buildRule());
      setState(() => _preview = preview);
    } on ApiException catch (error) {
      setState(() {
        _preview = null;
        _previewError = error.message;
      });
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _save() async {
    final rule = _buildRule();
    if (_previewError != null) {
      _snack('请先修正规则：$_previewError');
      return;
    }
    final pin = await _askPin();
    if (pin == null) return;
    setState(() => _busy = true);
    try {
      await ref.read(ruleRepositoryProvider).update(widget.childId, pin, rule);
      await ref.read(pendingRuleStoreProvider).clear(widget.childId);
      if (mounted) Navigator.of(context).pop(true);
    } on ApiException catch (error) {
      if (error.statusCode == null) {
        await ref.read(pendingRuleStoreProvider).save(widget.childId, rule);
        if (mounted) {
          setState(() {
            _pending = PendingRule(rule: rule, savedAt: DateTime.now());
          });
          _snack('网络不可用，规则已暂存；联网后请在页面顶部同步');
        }
      } else {
        _snack(error.message);
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _syncPending() async {
    final pending = _pending;
    if (pending == null) return;
    final pin = await _askPin();
    if (pin == null) return;
    setState(() => _syncing = true);
    try {
      await ref
          .read(ruleRepositoryProvider)
          .update(widget.childId, pin, pending.rule);
      await ref.read(pendingRuleStoreProvider).clear(widget.childId);
      if (!mounted) return;
      setState(() => _pending = null);
      ref.invalidate(ruleProvider(widget.childId));
      _snack('待同步规则已保存');
    } on ApiException catch (error) {
      _snack(error.message);
    } finally {
      if (mounted) setState(() => _syncing = false);
    }
  }

  Future<String?> _askPin() async {
    final controller = TextEditingController();
    final result = await showDialog<String>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('输入家长 PIN'),
        content: TextField(
          controller: controller,
          autofocus: true,
          obscureText: true,
          keyboardType: TextInputType.number,
          inputFormatters: [FilteringTextInputFormatter.digitsOnly],
          maxLength: 6,
          decoration: const InputDecoration(
            labelText: '家长 PIN',
            counterText: '',
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(context, controller.text.trim()),
            child: const Text('确认保存'),
          ),
        ],
      ),
    );
    controller.dispose();
    return result;
  }

  void _snack(String message) {
    ScaffoldMessenger.of(
      context,
    ).showSnackBar(SnackBar(content: Text(message)));
  }

  @override
  Widget build(BuildContext context) {
    return ListView(
      padding: const EdgeInsets.all(20),
      children: [
        if (_pending != null)
          Card(
            color: Colors.orange.shade50,
            margin: const EdgeInsets.only(bottom: 12),
            child: Padding(
              padding: const EdgeInsets.all(12),
              child: Row(
                children: [
                  Expanded(
                    child: Text(
                      '有未同步的规则修改（${_pending!.savedAt.month}-'
                      '${_pending!.savedAt.day} '
                      '${_pending!.savedAt.hour.toString().padLeft(2, '0')}:'
                      '${_pending!.savedAt.minute.toString().padLeft(2, '0')}）',
                      style: const TextStyle(fontSize: 13),
                    ),
                  ),
                  const SizedBox(width: 8),
                  FilledButton(
                    onPressed: _busy || _syncing ? null : _syncPending,
                    child: _syncing
                        ? const SizedBox(
                            width: 18,
                            height: 18,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : const Text('同步'),
                  ),
                ],
              ),
            ),
          ),
        if (widget.staleAt != null)
          Padding(
            padding: const EdgeInsets.only(bottom: 8),
            child: Text(
              '离线数据 · 最后同步 ${widget.staleAt!.month}-${widget.staleAt!.day} '
              '${widget.staleAt!.hour.toString().padLeft(2, '0')}:'
              '${widget.staleAt!.minute.toString().padLeft(2, '0')}',
              style: const TextStyle(color: Colors.orange, fontSize: 13),
            ),
          ),
        Text(
          '规则版本 v${widget.version}',
          style: Theme.of(context).textTheme.bodySmall,
        ),
        const SizedBox(height: 12),
        DropdownButtonFormField<String>(
          initialValue: _preset,
          decoration: const InputDecoration(labelText: '权重预设'),
          items: [
            for (final entry in presetLabels.entries)
              DropdownMenuItem(value: entry.key, child: Text(entry.value)),
          ],
          onChanged: _applyPreset,
        ),
        const SizedBox(height: 16),
        Row(
          children: [
            Expanded(child: _numberField(_unlock, '解锁时长（分钟）')),
            const SizedBox(width: 12),
            Expanded(child: _numberField(_daily, '每日上限（分钟）')),
          ],
        ),
        const SizedBox(height: 12),
        Row(
          children: [
            Expanded(child: _numberField(_threshold, '加权门槛')),
            const SizedBox(width: 12),
            Expanded(child: _numberField(_offline, '离线降级（分钟）')),
          ],
        ),
        const SizedBox(height: 12),
        SwitchListTile(
          contentPadding: EdgeInsets.zero,
          title: const Text('包含趣味题'),
          subtitle: const Text('趣味题权重需低于最低学科权重'),
          value: _funEnabled,
          onChanged: (value) => setState(() => _funEnabled = value),
        ),
        Row(children: [Expanded(child: _numberField(_funCap, '趣味题上限'))]),
        const SizedBox(height: 12),
        DropdownButtonFormField<String>(
          initialValue: _scope,
          decoration: const InputDecoration(labelText: '管控范围'),
          items: [
            for (final entry in scopeLabels.entries)
              DropdownMenuItem(value: entry.key, child: Text(entry.value)),
          ],
          onChanged: (value) => setState(() => _scope = value ?? 'media'),
        ),
        if (_scope == 'custom') ...[
          const SizedBox(height: 12),
          TextField(
            controller: _allowedApps,
            decoration: const InputDecoration(
              labelText: '允许的应用包名（逗号分隔）',
              hintText: 'com.tianci.movieplatform, com.qiyi.video',
            ),
          ),
        ],
        SwitchListTile(
          contentPadding: EdgeInsets.zero,
          title: const Text('限制观看时段'),
          subtitle: const Text('关闭表示不限时段'),
          value: _windowEnabled,
          onChanged: (value) => setState(() => _windowEnabled = value),
        ),
        if (_windowEnabled)
          Row(
            children: [
              Expanded(
                child: OutlinedButton(
                  onPressed: () => _pickTime(isStart: true),
                  child: Text('开始 ${_formatTime(_windowStart)}'),
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: OutlinedButton(
                  onPressed: () => _pickTime(isStart: false),
                  child: Text('结束 ${_formatTime(_windowEnd)}'),
                ),
              ),
            ],
          ),
        const SizedBox(height: 8),
        ExpansionTile(
          tilePadding: EdgeInsets.zero,
          title: const Text('进阶：学科权重'),
          children: [
            for (final entry in subjectLabels.entries)
              Padding(
                padding: const EdgeInsets.only(bottom: 10),
                child: Row(
                  children: [
                    SizedBox(width: 72, child: Text(entry.value)),
                    Expanded(
                      child: TextField(
                        controller: _weights[entry.key],
                        keyboardType: const TextInputType.numberWithOptions(
                          decimal: true,
                        ),
                        decoration: const InputDecoration(isDense: true),
                      ),
                    ),
                  ],
                ),
              ),
          ],
        ),
        const SizedBox(height: 16),
        if (_preview != null)
          Card(
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    '预览：${_preview!.questionCount} 题，最高加权分 '
                    '${_preview!.maxScore.toStringAsFixed(1)}',
                  ),
                  if (_preview!.warnings.isNotEmpty)
                    Padding(
                      padding: const EdgeInsets.only(top: 8),
                      child: Text(
                        _preview!.warnings.join('\n'),
                        style: const TextStyle(color: Colors.orange),
                      ),
                    ),
                ],
              ),
            ),
          ),
        if (_previewError != null)
          Padding(
            padding: const EdgeInsets.only(bottom: 8),
            child: Text(
              _previewError!,
              style: const TextStyle(color: Colors.red),
            ),
          ),
        const SizedBox(height: 8),
        Row(
          children: [
            Expanded(
              child: OutlinedButton(
                onPressed: _busy ? null : _previewRule,
                child: const Text('预览可达性'),
              ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: FilledButton(
                onPressed: _busy ? null : _save,
                child: _busy
                    ? const SizedBox(
                        width: 20,
                        height: 20,
                        child: CircularProgressIndicator(strokeWidth: 2),
                      )
                    : const Text('保存规则'),
              ),
            ),
          ],
        ),
      ],
    );
  }

  Widget _numberField(TextEditingController controller, String label) {
    return TextField(
      controller: controller,
      keyboardType: const TextInputType.numberWithOptions(decimal: true),
      decoration: InputDecoration(labelText: label),
    );
  }

  Future<void> _pickTime({required bool isStart}) async {
    final picked = await showTimePicker(
      context: context,
      initialTime: isStart ? _windowStart : _windowEnd,
    );
    if (picked == null) return;
    setState(() {
      if (isStart) {
        _windowStart = picked;
      } else {
        _windowEnd = picked;
      }
    });
  }

  TimeOfDay? _parseTime(String? value) {
    if (value == null) return null;
    final parts = value.split(':');
    if (parts.length != 2) return null;
    final hour = int.tryParse(parts[0]);
    final minute = int.tryParse(parts[1]);
    if (hour == null || minute == null) return null;
    return TimeOfDay(hour: hour, minute: minute);
  }

  String _formatTime(TimeOfDay time) =>
      '${time.hour.toString().padLeft(2, '0')}:'
      '${time.minute.toString().padLeft(2, '0')}';
}
