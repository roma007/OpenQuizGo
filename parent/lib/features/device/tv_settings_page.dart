import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/api_client.dart';
import '../../core/providers.dart';
import '../../data/tv_repository.dart';

/// 081：单台电视的配置页，全部配置只在这里改，电视端不再提供配置入口。
class TvSettingsPage extends ConsumerStatefulWidget {
  const TvSettingsPage({super.key, required this.device});

  final TvDevice device;

  @override
  ConsumerState<TvSettingsPage> createState() => _TvSettingsPageState();
}

class _TvSettingsPageState extends ConsumerState<TvSettingsPage> {
  static const _unlockOptions = [5, 10, 15, 20, 30, 45, 60, 90, 120];
  static const _limitOptions = [30, 45, 60, 90, 120, 150, 180, 240, 300, 360];
  static const _questionCountOptions = [1, 2, 3, 4, 5];

  late TvConfig _config;
  late final TextEditingController _nickname;
  String? _gradeTag;
  bool _savingConfig = false;
  bool _savingChild = false;

  @override
  void initState() {
    super.initState();
    _config = widget.device.config;
    _nickname = TextEditingController(
      text: widget.device.hasChild ? widget.device.childName : '',
    );
    _gradeTag = widget.device.gradeTag.isEmpty ? null : widget.device.gradeTag;
  }

  @override
  void dispose() {
    _nickname.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      appBar: AppBar(title: Text('${widget.device.name} 设置')),
      body: ListView(
        padding: const EdgeInsets.fromLTRB(16, 16, 16, 96),
        children: [
          Text(
            '所有设置只对这台${widget.device.typeLabel}生效，保存后 60 秒内下发。',
            style: theme.textTheme.bodySmall,
          ),
          const SizedBox(height: 12),
          _card(
            title: '做题',
            children: [
              SwitchListTile(
                contentPadding: EdgeInsets.zero,
                title: const Text('到点强制做题'),
                subtitle: Text(
                  _config.quizEnabled
                      ? '默认开启：每 ${_config.unlockMinutes} 分钟弹一次题，'
                            '每次 ${_config.questionCount} 题，'
                            '答对 ${_config.passQuestions} 题可继续看'
                      : '已关闭：不再弹题，但每日上限仍然生效',
                ),
                value: _config.quizEnabled,
                onChanged: (value) => setState(() {
                  _config = _config.copyWith(quizEnabled: value);
                }),
              ),
              _intDropdown(
                label: '强制答题间隔',
                value: _config.unlockMinutes,
                options: _unlockOptions,
                optionLabel: (value) => '$value 分钟',
                onChanged: (value) => setState(() {
                  _config = _config.copyWith(unlockMinutes: value);
                }),
              ),
              const SizedBox(height: 12),
              _intDropdown(
                label: '每次答题题量',
                value: _config.questionCount,
                options: _questionCountOptions,
                optionLabel: (value) => '$value 题',
                onChanged: (value) => setState(() {
                  var pass = _config.passQuestions;
                  if (pass > value) pass = value;
                  // 多题场次默认要求答对 2 题，避免题量调大后仍然 1 题就通过。
                  if (pass < 2 && value >= 2) pass = 2;
                  _config = _config.copyWith(
                    questionCount: value,
                    passQuestions: pass,
                  );
                }),
              ),
              const SizedBox(height: 4),
              Text('到点后需要答完这么多题', style: theme.textTheme.bodySmall),
              const SizedBox(height: 12),
              _intDropdown(
                label: '答对几题算通过',
                value: _config.passQuestions,
                options: [for (var i = 1; i <= _config.questionCount; i++) i],
                optionLabel: (value) => '$value 题',
                onChanged: (value) => setState(() {
                  _config = _config.copyWith(passQuestions: value);
                }),
              ),
              const SizedBox(height: 4),
              Text('答对这么多题后，设备继续播放', style: theme.textTheme.bodySmall),
              const SizedBox(height: 12),
              Text('题目范围', style: theme.textTheme.bodySmall),
              const SizedBox(height: 8),
              SegmentedButton<String>(
                segments: const [
                  ButtonSegment(value: 'random', label: Text('全库随机')),
                  ButtonSegment(value: 'grade', label: Text('按孩子年级')),
                ],
                selected: {_config.quizScope},
                onSelectionChanged: (selection) => setState(() {
                  _config = _config.copyWith(quizScope: selection.first);
                }),
              ),
              if (_config.quizScope == 'grade' && (_gradeTag ?? '').isEmpty)
                const Padding(
                  padding: EdgeInsets.only(top: 8),
                  child: Text(
                    '还没有设置年级，保存前请先在下方填写孩子信息',
                    style: TextStyle(color: Colors.orange),
                  ),
                ),
              const SizedBox(height: 12),
              Text('出题学科', style: theme.textTheme.bodySmall),
              const SizedBox(height: 8),
              Wrap(
                spacing: 8,
                runSpacing: 4,
                children: [
                  for (final entry in subjectLabels.entries)
                    FilterChip(
                      label: Text(entry.value),
                      selected: _config.subjects.contains(entry.key),
                      onSelected: (selected) => setState(() {
                        final next = {..._config.subjects};
                        if (selected) {
                          next.add(entry.key);
                        } else {
                          next.remove(entry.key);
                        }
                        _config = _config.copyWith(
                          subjects: next.toList(growable: false),
                        );
                      }),
                    ),
                ],
              ),
              const SizedBox(height: 6),
              Text('不选表示使用孩子规则里的全部学科', style: theme.textTheme.bodySmall),
            ],
          ),
          const SizedBox(height: 12),
          _card(
            title: '观看时长',
            children: [
              SwitchListTile(
                contentPadding: EdgeInsets.zero,
                title: const Text('每日观看上限'),
                subtitle: Text(
                  _config.limitEnabled
                      ? '每天最多看 ${_config.dailyLimit} 分钟，用完立即硬停'
                      : '已关闭：今天不限制观看时长，仍会按间隔弹题',
                ),
                value: _config.limitEnabled,
                onChanged: (value) => setState(() {
                  _config = _config.copyWith(limitEnabled: value);
                }),
              ),
              _intDropdown(
                label: '每日上限',
                value: _config.dailyLimit,
                options: _limitOptions,
                optionLabel: (value) => '$value 分钟',
                onChanged: (value) => setState(() {
                  _config = _config.copyWith(dailyLimit: value);
                }),
              ),
              const SizedBox(height: 12),
              _intDropdown(
                label: '家长 PIN 放行时长',
                value: _config.hardStopGrace,
                options: _unlockOptions,
                optionLabel: (value) => '$value 分钟',
                onChanged: (value) => setState(() {
                  _config = _config.copyWith(hardStopGrace: value);
                }),
              ),
              const SizedBox(height: 4),
              Text(
                '硬停后在设备上输入家长 PIN，可临时放行这么久',
                style: theme.textTheme.bodySmall,
              ),
            ],
          ),
          const SizedBox(height: 12),
          _card(
            title: '孩子',
            children: [
              TextField(
                controller: _nickname,
                decoration: const InputDecoration(
                  labelText: '昵称',
                  hintText: '例如：小明',
                ),
                maxLength: 20,
              ),
              const SizedBox(height: 4),
              DropdownButtonFormField<String?>(
                initialValue: _gradeTag,
                decoration: const InputDecoration(labelText: '年级'),
                items: [
                  const DropdownMenuItem<String?>(
                    value: null,
                    child: Text('不指定年级'),
                  ),
                  for (final entry in gradeLabels.entries)
                    DropdownMenuItem<String?>(
                      value: entry.key,
                      child: Text(entry.value),
                    ),
                ],
                onChanged: (value) => setState(() => _gradeTag = value),
              ),
              const SizedBox(height: 6),
              Text(
                '每年 9 月 1 日自动升一年级（封顶九年级）；'
                '不指定孩子时按全库随机出题。',
                style: theme.textTheme.bodySmall,
              ),
              const SizedBox(height: 12),
              Align(
                alignment: Alignment.centerRight,
                child: OutlinedButton.icon(
                  onPressed: _savingChild ? null : _saveChild,
                  icon: const Icon(Icons.badge_outlined),
                  label: Text(_savingChild ? '保存中…' : '保存孩子信息'),
                ),
              ),
            ],
          ),
        ],
      ),
      bottomNavigationBar: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: FilledButton.icon(
            onPressed: _savingConfig ? null : _saveConfig,
            icon: const Icon(Icons.save),
            label: Text(_savingConfig ? '保存中…' : '保存并下发到设备'),
          ),
        ),
      ),
    );
  }

  Widget _card({required String title, required List<Widget> children}) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(title, style: Theme.of(context).textTheme.titleMedium),
            const SizedBox(height: 4),
            ...children,
          ],
        ),
      ),
    );
  }

  Widget _intDropdown({
    required String label,
    required int value,
    required List<int> options,
    required String Function(int value) optionLabel,
    required void Function(int value) onChanged,
  }) {
    final values = {...options, value}.toList(growable: false)..sort();
    return DropdownButtonFormField<int>(
      initialValue: value,
      decoration: InputDecoration(labelText: label),
      items: [
        for (final item in values)
          DropdownMenuItem<int>(value: item, child: Text(optionLabel(item))),
      ],
      onChanged: (selected) {
        if (selected != null) onChanged(selected);
      },
    );
  }

  Future<void> _saveChild() async {
    final nickname = _nickname.text.trim();
    if (nickname.isEmpty) {
      _toast('请先填写孩子昵称');
      return;
    }
    if (_config.quizScope == 'grade' && (_gradeTag ?? '').isEmpty) {
      _toast('按年级出题需要选择年级');
      return;
    }
    setState(() => _savingChild = true);
    try {
      final updated = await ref
          .read(tvRepositoryProvider)
          .assignChild(
            deviceId: widget.device.id,
            nickname: nickname,
            gradeTag: _gradeTag,
          );
      ref.invalidate(tvsProvider);
      setState(() {
        _gradeTag = updated.gradeTag.isEmpty ? null : updated.gradeTag;
      });
      _toast('孩子信息已保存');
    } on ApiException catch (error) {
      _toast(error.message);
    } finally {
      if (mounted) setState(() => _savingChild = false);
    }
  }

  Future<void> _saveConfig() async {
    if (_config.quizScope == 'grade' && (_gradeTag ?? '').isEmpty) {
      _toast('按年级出题需要先保存孩子年级');
      return;
    }
    final pin = await _askPin();
    if (pin == null || pin.isEmpty) return;
    setState(() => _savingConfig = true);
    try {
      await ref
          .read(tvRepositoryProvider)
          .updateConfig(deviceId: widget.device.id, pin: pin, config: _config);
      ref.invalidate(tvsProvider);
      if (!mounted) return;
      Navigator.of(context).pop(true);
    } on ApiException catch (error) {
      if (error.code == 'TV_CONFIG_CONFLICT') {
        await _reloadConfig();
        _toast('配置已被其他家长修改，已刷新为最新配置，请重新保存');
      } else {
        _toast(error.message);
      }
    } finally {
      if (mounted) setState(() => _savingConfig = false);
    }
  }

  Future<void> _reloadConfig() async {
    try {
      final latest = await ref
          .read(tvRepositoryProvider)
          .getConfig(widget.device.id);
      if (mounted) setState(() => _config = latest);
    } on ApiException {
      // 刷新失败保留本地值，用户可手动重试。
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
            child: const Text('保存'),
          ),
        ],
      ),
    );
    controller.dispose();
    return result;
  }

  void _toast(String message) {
    if (!mounted) return;
    ScaffoldMessenger.of(context)
      ..hideCurrentSnackBar()
      ..showSnackBar(SnackBar(content: Text(message)));
  }
}
