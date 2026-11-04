import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/api_client.dart';
import '../../core/api/models.dart';
import '../../core/providers.dart';
import '../../data/family_repository.dart';

class ChildEditPage extends ConsumerStatefulWidget {
  const ChildEditPage({super.key, this.child});

  final Child? child;

  @override
  ConsumerState<ChildEditPage> createState() => _ChildEditPageState();
}

class _ChildEditPageState extends ConsumerState<ChildEditPage> {
  final _formKey = GlobalKey<FormState>();
  late final TextEditingController _nickname;
  late String _stage;
  int? _birthYear;
  int? _birthMonth;
  bool _saving = false;

  static const _stages = ['preschool', 'low', 'mid', 'high'];

  @override
  void initState() {
    super.initState();
    final child = widget.child;
    _nickname = TextEditingController(text: child?.nickname ?? '');
    _stage = child?.stage ?? 'low';
    _birthYear = child?.birthYear;
    _birthMonth = child?.birthMonth;
  }

  @override
  void dispose() {
    _nickname.dispose();
    super.dispose();
  }

  Future<void> _save() async {
    if (!_formKey.currentState!.validate()) return;
    setState(() => _saving = true);
    final draft = ChildDraft(
      nickname: _nickname.text.trim(),
      stage: _stage,
      birthYear: _birthYear,
      birthMonth: _birthMonth,
      gradeTag: widget.child?.gradeTag,
    );
    try {
      final repository = ref.read(familyRepositoryProvider);
      if (widget.child == null) {
        await repository.createChild(draft);
      } else {
        await repository.updateChild(widget.child!.id, draft);
      }
      if (mounted) Navigator.of(context).pop(true);
    } on ApiException catch (error) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(error.message)));
      }
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final years = [for (var y = 2008; y <= 2026; y++) y];
    return Scaffold(
      appBar: AppBar(title: Text(widget.child == null ? '新增孩子' : '编辑孩子')),
      body: Form(
        key: _formKey,
        child: ListView(
          padding: const EdgeInsets.all(20),
          children: [
            TextFormField(
              controller: _nickname,
              maxLength: 20,
              decoration: const InputDecoration(
                labelText: '昵称',
                hintText: '例如：小明',
                counterText: '',
              ),
              validator: (value) {
                final name = value?.trim() ?? '';
                if (name.isEmpty) return '请输入昵称';
                if (name.length > 20) return '昵称最多 20 个字';
                return null;
              },
            ),
            const SizedBox(height: 16),
            DropdownButtonFormField<String>(
              initialValue: _stage,
              decoration: const InputDecoration(labelText: '学段'),
              items: [
                for (final stage in _stages)
                  DropdownMenuItem(
                    value: stage,
                    child: Text(stageLabel(stage)),
                  ),
              ],
              onChanged: (value) => setState(() => _stage = value ?? 'low'),
            ),
            const SizedBox(height: 16),
            Row(
              children: [
                Expanded(
                  child: DropdownButtonFormField<int>(
                    initialValue: _birthYear,
                    decoration: const InputDecoration(labelText: '出生年份（可选）'),
                    items: [
                      for (final year in years)
                        DropdownMenuItem(value: year, child: Text('$year 年')),
                    ],
                    onChanged: (value) => setState(() => _birthYear = value),
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: DropdownButtonFormField<int>(
                    initialValue: _birthMonth,
                    decoration: const InputDecoration(labelText: '出生月份'),
                    items: [
                      for (var month = 1; month <= 12; month++)
                        DropdownMenuItem(value: month, child: Text('$month 月')),
                    ],
                    onChanged: (value) => setState(() => _birthMonth = value),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 12),
            SwitchListTile(
              contentPadding: EdgeInsets.zero,
              title: const Text('已设置生日'),
              subtitle: const Text('用于按年龄匹配题目；关闭则清空'),
              value: _birthYear != null,
              onChanged: (value) {
                setState(() {
                  if (value) {
                    _birthYear ??= 2018;
                    _birthMonth ??= 1;
                  } else {
                    _birthYear = null;
                    _birthMonth = null;
                  }
                });
              },
            ),
            const SizedBox(height: 28),
            FilledButton(
              onPressed: _saving ? null : _save,
              child: _saving
                  ? const SizedBox(
                      width: 20,
                      height: 20,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : const Text('保存'),
            ),
          ],
        ),
      ),
    );
  }
}
