#!/usr/bin/env node
// 版本自增：1.0.X → 1.0.(X+1)
//
// 铁律 3：每次提交必须升级版本（1.0.X）。本脚本由 scripts/git-hooks/pre-commit
// 自动调用，也可手动执行：
//
//   node scripts/bump-version.mjs            # 写入新版本
//   node scripts/bump-version.mjs --dry-run  # 只打印，不落盘
//
// 同步位置：
//   VERSION                        → 1.0.X
//   tv/app/build.gradle.kts        → versionName = "1.0.X"，versionCode = 100 + X
//   parent/pubspec.yaml            → version: 1.0.X+<100 + X>

import { execFileSync } from 'node:child_process';
import { readFileSync, writeFileSync } from 'node:fs';

const dryRun = process.argv.includes('--dry-run');
const root = execFileSync('git', ['rev-parse', '--show-toplevel'], {
  encoding: 'utf8',
}).trim();

const read = (path) => readFileSync(`${root}/${path}`, 'utf8');
const write = (path, content) => {
  if (!dryRun) writeFileSync(`${root}/${path}`, content);
};

const current = read('VERSION').trim();
const matched = /^1\.0\.(\d+)$/.exec(current);
if (!matched) {
  console.error(`VERSION 格式非法：${current}（必须为 1.0.X）`);
  process.exit(1);
}

const patch = Number(matched[1]) + 1;
const version = `1.0.${patch}`;
const build = 100 + patch;

const tvPath = 'tv/app/build.gradle.kts';
let tv = read(tvPath);
if (!/versionCode = \d+/.test(tv) || !/versionName = "[^"]*"/.test(tv)) {
  console.error(`${tvPath} 中未找到 versionCode / versionName`);
  process.exit(1);
}
tv = tv
  .replace(/versionCode = \d+/, `versionCode = ${build}`)
  .replace(/versionName = "[^"]*"/, `versionName = "${version}"`);

const pubspecPath = 'parent/pubspec.yaml';
let pubspec = read(pubspecPath);
if (!/^version: .+$/m.test(pubspec)) {
  console.error(`${pubspecPath} 中未找到 version`);
  process.exit(1);
}
pubspec = pubspec.replace(/^version: .+$/m, `version: ${version}+${build}`);

if (dryRun) {
  console.log(`[dry-run] ${current} -> ${version}（build ${build}）`);
} else {
  write('VERSION', `${version}\n`);
  write(tvPath, tv);
  write(pubspecPath, pubspec);
  console.log(`${current} -> ${version}`);
}
