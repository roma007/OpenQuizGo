#!/usr/bin/env swift
// 生成 iOS App 图标与启动图：把 docs/brand/logo-1024.png 压平到品牌深色底并缩放。
// 用法（在 parent 目录执行）：swift tool/generate_ios_assets.swift
import AppKit
import Foundation
import ImageIO
import UniformTypeIdentifiers

let fileManager = FileManager.default
let parentDir = URL(fileURLWithPath: fileManager.currentDirectoryPath)
let logoURL = parentDir.appendingPathComponent("../docs/brand/logo-1024.png")
let iconDir = parentDir.appendingPathComponent(
  "ios/Runner/Assets.xcassets/AppIcon.appiconset")
let launchDir = parentDir.appendingPathComponent(
  "ios/Runner/Assets.xcassets/LaunchImage.imageset")

guard let logoImage = NSImage(contentsOf: logoURL),
  let tiff = logoImage.tiffRepresentation,
  let sourceRep = NSBitmapImageRep(data: tiff),
  let logoCG = logoImage.cgImage(forProposedRect: nil, context: nil, hints: nil)
else {
  fputs("无法读取 Logo：\(logoURL.path)\n", stderr)
  exit(1)
}

// 取 Logo 顶部中间像素作为压平底色（Logo 自带深色圆角背景）。
guard let background = sourceRep.colorAt(x: sourceRep.pixelsWide / 2, y: 60)?
  .withAlphaComponent(1) else {
  fputs("无法读取 Logo 像素\n", stderr)
  exit(1)
}

func writePNG(
  size: Int, to url: URL, opaque: Bool
) throws {
  let colorSpace = CGColorSpaceCreateDeviceRGB()
  let alphaInfo: CGImageAlphaInfo = opaque ? .noneSkipLast : .premultipliedLast
  guard
    let context = CGContext(
      data: nil,
      width: size,
      height: size,
      bitsPerComponent: 8,
      bytesPerRow: 0,
      space: colorSpace,
      bitmapInfo: alphaInfo.rawValue
    )
  else {
    throw NSError(domain: "assets", code: 1)
  }
  context.interpolationQuality = .high
  let rect = CGRect(x: 0, y: 0, width: size, height: size)
  if opaque {
    context.setFillColor(background.cgColor)
    context.fill(rect)
  } else {
    context.clear(rect)
  }
  context.draw(logoCG, in: rect)
  guard let image = context.makeImage(),
    let destination = CGImageDestinationCreateWithURL(
      url as CFURL, UTType.png.identifier as CFString, 1, nil)
  else {
    throw NSError(domain: "assets", code: 2)
  }
  CGImageDestinationAddImage(destination, image, nil)
  guard CGImageDestinationFinalize(destination) else {
    throw NSError(domain: "assets", code: 3)
  }
}

let icons: [(String, Int)] = [
  ("Icon-App-20x20@1x.png", 20),
  ("Icon-App-20x20@2x.png", 40),
  ("Icon-App-20x20@3x.png", 60),
  ("Icon-App-29x29@1x.png", 29),
  ("Icon-App-29x29@2x.png", 58),
  ("Icon-App-29x29@3x.png", 87),
  ("Icon-App-40x40@1x.png", 40),
  ("Icon-App-40x40@2x.png", 80),
  ("Icon-App-40x40@3x.png", 120),
  ("Icon-App-60x60@2x.png", 120),
  ("Icon-App-60x60@3x.png", 180),
  ("Icon-App-76x76@1x.png", 76),
  ("Icon-App-76x76@2x.png", 152),
  ("Icon-App-83.5x83.5@2x.png", 167),
  ("Icon-App-1024x1024@1x.png", 1024),
]

for (name, size) in icons {
  try writePNG(size: size, to: iconDir.appendingPathComponent(name), opaque: true)
}

let launchImages: [(String, Int)] = [
  ("LaunchImage.png", 168),
  ("LaunchImage@2x.png", 336),
  ("LaunchImage@3x.png", 504),
]
for (name, size) in launchImages {
  try writePNG(
    size: size, to: launchDir.appendingPathComponent(name), opaque: false)
}

print(
  "已生成 \(icons.count) 张 App 图标与 \(launchImages.count) 张启动图，底色 \(background)")
