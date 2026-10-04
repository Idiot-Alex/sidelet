// Deterministic renderer for our rect/polyline SVG artwork. No GUI or external dependencies.
import AppKit
import Foundation

final class Icon: NSObject, XMLParserDelegate {
    var elements: [(String, [String: String])] = []
    func parser(_ parser: XMLParser, didStartElement name: String, namespaceURI: String?, qualifiedName: String?, attributes: [String: String]) {
        if name == "rect" || name == "polyline" { elements.append((name, attributes)) }
        else if name != "svg" { fatalError("Unsupported SVG element: \(name)") }
    }
    func color(_ hex: String) -> NSColor {
        let number = UInt32(hex.dropFirst(), radix: 16)!
        return NSColor(srgbRed: CGFloat((number >> 16) & 255) / 255, green: CGFloat((number >> 8) & 255) / 255, blue: CGFloat(number & 255) / 255, alpha: 1)
    }
    func render(_ pixels: Int, to url: URL) throws {
        let bitmap = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: pixels, pixelsHigh: pixels, bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false, colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0)!
        let context = NSGraphicsContext(bitmapImageRep: bitmap)!
        NSGraphicsContext.saveGraphicsState()
        NSGraphicsContext.current = context
        context.imageInterpolation = .high
        let transform = NSAffineTransform()
        transform.translateX(by: 0, yBy: CGFloat(pixels))
        transform.scaleX(by: CGFloat(pixels) / 1024, yBy: -CGFloat(pixels) / 1024)
        transform.concat()
        for (name, a) in elements {
            func value(_ key: String) -> CGFloat { CGFloat(Double(a[key]!)!) }
            if name == "rect" {
                color(a["fill"]!).setFill()
                NSBezierPath(roundedRect: NSRect(x: value("x"), y: value("y"), width: value("width"), height: value("height")), xRadius: value("rx"), yRadius: value("rx")).fill()
            } else {
                let points = a["points"]!.split(separator: " ").map { pair -> NSPoint in
                    let p = pair.split(separator: ",").map { CGFloat(Double($0)!) }
                    return NSPoint(x: p[0], y: p[1])
                }
                let path = NSBezierPath()
                path.move(to: points[0]); for point in points.dropFirst() { path.line(to: point) }
                path.lineWidth = value("stroke-width"); path.lineCapStyle = .round; path.lineJoinStyle = .round
                color(a["stroke"]!).setStroke(); path.stroke()
            }
        }
        NSGraphicsContext.restoreGraphicsState()
        try bitmap.representation(using: .png, properties: [:])!.write(to: url)
    }
}
if CommandLine.arguments.count != 3 { fatalError("usage: render-macos-icon artwork.svg output.iconset") }
let source = URL(fileURLWithPath: CommandLine.arguments[1])
let output = URL(fileURLWithPath: CommandLine.arguments[2], isDirectory: true)
let icon = Icon(); let parser = XMLParser(contentsOf: source)!; parser.delegate = icon
if !parser.parse() { fatalError("Invalid SVG") }
try FileManager.default.createDirectory(at: output, withIntermediateDirectories: true)
for size in [16, 32, 128, 256, 512] {
    try icon.render(size, to: output.appendingPathComponent("icon_\(size)x\(size).png"))
    try icon.render(size * 2, to: output.appendingPathComponent("icon_\(size)x\(size)@2x.png"))
}
