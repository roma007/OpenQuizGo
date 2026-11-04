import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/intl.dart';

import '../../core/api/models.dart';
import '../../core/providers.dart';
import '../../data/report_repository.dart';
import '../../data/rule_repository.dart';

class ReportPage extends ConsumerStatefulWidget {
  const ReportPage({super.key});

  @override
  ConsumerState<ReportPage> createState() => _ReportPageState();
}

class _ReportPageState extends ConsumerState<ReportPage> {
  String? _childId;
  String _period = 'week';

  @override
  Widget build(BuildContext context) {
    final children = ref.watch(childrenProvider);
    return children.when(
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (error, _) => Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text('加载孩子失败：$error'),
            const SizedBox(height: 12),
            OutlinedButton(
              onPressed: () => ref.invalidate(childrenProvider),
              child: const Text('重试'),
            ),
          ],
        ),
      ),
      data: (cached) {
        final items = cached.data;
        if (items.isEmpty) {
          return const Center(child: Text('还没有孩子档案，创建后即可查看报告'));
        }
        final child = _pickChild(items);
        return Column(
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 12, 16, 8),
              child: Row(
                children: [
                  Expanded(
                    child: DropdownButtonFormField<String>(
                      initialValue: child.id,
                      decoration: const InputDecoration(
                        labelText: '孩子',
                        isDense: true,
                      ),
                      items: [
                        for (final item in items)
                          DropdownMenuItem(
                            value: item.id,
                            child: Text(item.nickname),
                          ),
                      ],
                      onChanged: (value) =>
                          setState(() => _childId = value ?? child.id),
                    ),
                  ),
                  const SizedBox(width: 12),
                  SegmentedButton<String>(
                    segments: const [
                      ButtonSegment(value: 'week', label: Text('周')),
                      ButtonSegment(value: 'month', label: Text('月')),
                    ],
                    selected: {_period},
                    onSelectionChanged: (selection) =>
                        setState(() => _period = selection.first),
                  ),
                ],
              ),
            ),
            Expanded(
              child: _ReportBody(
                query: ReportQuery(childId: child.id, period: _period),
                childName: child.nickname,
              ),
            ),
          ],
        );
      },
    );
  }

  Child _pickChild(List<Child> items) {
    if (_childId == null) return items.first;
    return items.firstWhere(
      (item) => item.id == _childId,
      orElse: () => items.first,
    );
  }
}

String _formatCachedAt(DateTime? at) {
  if (at == null) return '';
  return '${at.month}-${at.day} '
      '${at.hour.toString().padLeft(2, '0')}:'
      '${at.minute.toString().padLeft(2, '0')}';
}

class _ReportBody extends ConsumerWidget {
  const _ReportBody({required this.query, required this.childName});

  final ReportQuery query;
  final String childName;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final reports = ref.watch(reportsProvider(query));
    return reports.when(
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (error, _) => Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text('加载报告失败：$error'),
            const SizedBox(height: 12),
            OutlinedButton(
              onPressed: () => ref.invalidate(reportsProvider(query)),
              child: const Text('重试'),
            ),
          ],
        ),
      ),
      data: (cachedReports) {
        final items = cachedReports.data;
        if (items.isEmpty) {
          return Center(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(
                  Icons.insights,
                  size: 64,
                  color: Theme.of(context).colorScheme.primary,
                ),
                const SizedBox(height: 16),
                Text('$childName 还没有${query.period == 'week' ? '本周' : '本月'}报告'),
                const SizedBox(height: 8),
                const Text('完成一次答题后，这里会展示答题与观看统计'),
              ],
            ),
          );
        }
        return ListView(
          padding: const EdgeInsets.fromLTRB(16, 4, 16, 24),
          children: [
            if (cachedReports.stale)
              Padding(
                padding: const EdgeInsets.only(bottom: 8),
                child: Text(
                  '离线数据 · 最后同步 ${_formatCachedAt(cachedReports.cachedAt)}',
                  style: const TextStyle(color: Colors.orange, fontSize: 13),
                ),
              ),
            _LatestReportCard(report: items.first),
            if (items.length > 1) ...[
              const SizedBox(height: 16),
              Text('历史报告', style: Theme.of(context).textTheme.titleMedium),
              const SizedBox(height: 8),
              for (final report in items.skip(1)) _HistoryTile(report: report),
            ],
          ],
        );
      },
    );
  }
}

class _LatestReportCard extends StatelessWidget {
  const _LatestReportCard({required this.report});

  final GrowthReport report;

  @override
  Widget build(BuildContext context) {
    final dateFormat = DateFormat('MM-dd');
    final range =
        '${report.periodStart == null ? '' : dateFormat.format(report.periodStart!.toLocal())}'
        ' - '
        '${report.periodEnd == null ? '' : dateFormat.format(report.periodEnd!.toLocal())}';
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(20),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(
                  child: Text(
                    range,
                    style: Theme.of(context).textTheme.titleMedium,
                  ),
                ),
                Text(
                  report.period == 'week' ? '周报' : '月报',
                  style: Theme.of(context).textTheme.bodySmall,
                ),
              ],
            ),
            const SizedBox(height: 16),
            Row(
              children: [
                _StatTile(
                  label: '观看',
                  value: '${report.watchMinutes}',
                  unit: '分钟',
                ),
                _StatTile(label: '答题', value: '${report.answered}', unit: '题'),
                _StatTile(
                  label: '正确率',
                  value: (report.accuracy * 100).toStringAsFixed(0),
                  unit: '%',
                ),
              ],
            ),
            const SizedBox(height: 16),
            if (report.subjectStats.isEmpty && report.funStats.answered == 0)
              const Text('本期没有答题记录')
            else
              ..._subjectRows(report),
          ],
        ),
      ),
    );
  }

  List<Widget> _subjectRows(GrowthReport report) {
    final rows = <Widget>[];
    final entries = report.subjectStats.entries.toList()
      ..sort((a, b) => b.value.answered.compareTo(a.value.answered));
    for (final entry in entries) {
      rows.add(
        _SubjectBar(
          label: subjectLabels[entry.key] ?? entry.key,
          stat: entry.value,
        ),
      );
    }
    if (report.funStats.answered > 0) {
      rows.add(_SubjectBar(label: '趣味', stat: report.funStats));
    }
    return rows;
  }
}

class _SubjectBar extends StatelessWidget {
  const _SubjectBar({required this.label, required this.stat});

  final String label;
  final SubjectStat stat;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              SizedBox(width: 56, child: Text(label)),
              Expanded(
                child: LinearProgressIndicator(
                  value: stat.accuracy.clamp(0, 1),
                  minHeight: 8,
                  borderRadius: BorderRadius.circular(4),
                ),
              ),
              const SizedBox(width: 12),
              Text(
                '${stat.correct}/${stat.answered} '
                '(${(stat.accuracy * 100).toStringAsFixed(0)}%)',
                style: Theme.of(context).textTheme.bodySmall,
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _StatTile extends StatelessWidget {
  const _StatTile({
    required this.label,
    required this.value,
    required this.unit,
  });

  final String label;
  final String value;
  final String unit;

  @override
  Widget build(BuildContext context) {
    return Expanded(
      child: Column(
        children: [
          Text(value, style: Theme.of(context).textTheme.headlineSmall),
          Text('$label（$unit）', style: Theme.of(context).textTheme.bodySmall),
        ],
      ),
    );
  }
}

class _HistoryTile extends StatelessWidget {
  const _HistoryTile({required this.report});

  final GrowthReport report;

  @override
  Widget build(BuildContext context) {
    final dateFormat = DateFormat('MM-dd');
    final range =
        '${report.periodStart == null ? '' : dateFormat.format(report.periodStart!.toLocal())}'
        ' - '
        '${report.periodEnd == null ? '' : dateFormat.format(report.periodEnd!.toLocal())}';
    return Card(
      child: ListTile(
        title: Text(range),
        subtitle: Text(
          '答题 ${report.answered} · 正确率 '
          '${(report.accuracy * 100).toStringAsFixed(0)}% · 观看 ${report.watchMinutes} 分钟',
        ),
      ),
    );
  }
}
