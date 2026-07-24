// SPDX-License-Identifier: MIT
//
// Paper Agent font rasterization. Skeleton thinning and path tracing live in
// `skeleton.rs`, adapted from smart_remarkable. This module deliberately owns
// only font selection, layout measurement and glyph rasterization.

use crate::skeleton;
use ab_glyph::{Font, FontRef, Glyph, GlyphId, PxScale, ScaleFont};
use std::sync::OnceLock;

const LATIN_TTF: &[u8] = include_bytes!("../assets/fonts/Kalam-Regular.ttf");
const CJK_HAND_TTF: &[u8] = include_bytes!("../assets/fonts/ChenYuluoyan-2.0-Thin.ttf");
const CJK_FALLBACK_TTF: &[u8] = include_bytes!("../assets/fonts/jf-openhuninn-2.1.ttf");
const SYMBOL_TTF: &[u8] = include_bytes!("../assets/fonts/NotoSansSymbols2-Regular.ttf");
const DEFAULT_CJK_SCALE: f32 = 0.70;
const RASTER_PAD: f32 = 6.0;

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
enum Face {
    Latin,
    CjkHand,
    CjkFallback,
    Symbols,
}

fn latin() -> &'static FontRef<'static> {
    static FONT: OnceLock<FontRef<'static>> = OnceLock::new();
    FONT.get_or_init(|| FontRef::try_from_slice(LATIN_TTF).expect("embedded Kalam font"))
}

fn cjk_hand() -> &'static FontRef<'static> {
    static FONT: OnceLock<FontRef<'static>> = OnceLock::new();
    FONT.get_or_init(|| FontRef::try_from_slice(CJK_HAND_TTF).expect("embedded ChenYuLuoyan font"))
}

fn cjk_fallback() -> &'static FontRef<'static> {
    static FONT: OnceLock<FontRef<'static>> = OnceLock::new();
    FONT.get_or_init(|| {
        FontRef::try_from_slice(CJK_FALLBACK_TTF).expect("embedded Open Huninn font")
    })
}

fn symbols() -> &'static FontRef<'static> {
    static FONT: OnceLock<FontRef<'static>> = OnceLock::new();
    FONT.get_or_init(|| {
        FontRef::try_from_slice(SYMBOL_TTF).expect("embedded Noto Sans Symbols 2 font")
    })
}

fn cjk_scale() -> f32 {
    std::env::var("PAPER_AGENT_CJK_SCALE")
        .ok()
        .and_then(|value| value.trim().parse::<f32>().ok())
        .filter(|value| value.is_finite())
        .unwrap_or(DEFAULT_CJK_SCALE)
        .clamp(0.55, 1.0)
}

pub fn supports_cjk_char(ch: char) -> bool {
    cjk_hand().glyph_id(ch).0 != 0 || cjk_fallback().glyph_id(ch).0 != 0
}

fn face_for(ch: char, px: f32) -> (Face, GlyphId, f32) {
    let is_cjk = is_cjk(ch);
    if !is_cjk {
        let latin_id = latin().glyph_id(ch);
        if latin_id.0 != 0 {
            return (Face::Latin, latin_id, px);
        }
        let symbol_id = symbols().glyph_id(ch);
        if symbol_id.0 != 0 {
            return (Face::Symbols, symbol_id, px);
        }
    }

    let hand_px = if is_cjk {
        (px * cjk_scale()).max(1.0)
    } else {
        px
    };
    let hand_id = cjk_hand().glyph_id(ch);
    if hand_id.0 != 0 {
        return (Face::CjkHand, hand_id, hand_px);
    }

    if is_cjk {
        let fallback_id = cjk_fallback().glyph_id(ch);
        if fallback_id.0 != 0 {
            return (Face::CjkFallback, fallback_id, hand_px);
        }
    }

    let latin_id = latin().glyph_id(ch);
    if latin_id.0 != 0 {
        return (Face::Latin, latin_id, px);
    }

    let fallback_id = cjk_fallback().glyph_id(ch);
    if fallback_id.0 != 0 {
        return (Face::CjkFallback, fallback_id, hand_px);
    }
    let symbol_id = symbols().glyph_id(ch);
    if symbol_id.0 != 0 {
        return (Face::Symbols, symbol_id, px);
    }
    (Face::Latin, latin().glyph_id('?'), px)
}

fn advance(face: Face, id: GlyphId, px: f32) -> f32 {
    match face {
        Face::Latin => latin().as_scaled(PxScale::from(px)).h_advance(id),
        Face::CjkHand => cjk_hand().as_scaled(PxScale::from(px)).h_advance(id),
        Face::CjkFallback => cjk_fallback().as_scaled(PxScale::from(px)).h_advance(id),
        Face::Symbols => symbols().as_scaled(PxScale::from(px)).h_advance(id),
    }
}

fn kern(face: Face, left: GlyphId, right: GlyphId, px: f32) -> f32 {
    match face {
        Face::Latin => latin().as_scaled(PxScale::from(px)).kern(left, right),
        Face::CjkHand => cjk_hand().as_scaled(PxScale::from(px)).kern(left, right),
        Face::CjkFallback => cjk_fallback()
            .as_scaled(PxScale::from(px))
            .kern(left, right),
        Face::Symbols => symbols().as_scaled(PxScale::from(px)).kern(left, right),
    }
}

fn metrics(face: Face, px: f32) -> (f32, f32) {
    match face {
        Face::Latin => {
            let scaled = latin().as_scaled(PxScale::from(px));
            (scaled.ascent(), scaled.descent())
        }
        Face::CjkHand => {
            let scaled = cjk_hand().as_scaled(PxScale::from(px));
            (scaled.ascent(), scaled.descent())
        }
        Face::CjkFallback => {
            let scaled = cjk_fallback().as_scaled(PxScale::from(px));
            (scaled.ascent(), scaled.descent())
        }
        Face::Symbols => {
            let scaled = symbols().as_scaled(PxScale::from(px));
            (scaled.ascent(), scaled.descent())
        }
    }
}

pub fn measure(text: &str, px: f32) -> f32 {
    let mut width = 0.0;
    let mut previous: Option<(Face, GlyphId, f32)> = None;
    for ch in text.chars() {
        let (face, id, glyph_px) = face_for(ch, px);
        if let Some((previous_face, previous_id, previous_px)) = previous {
            if previous_face == face && previous_px.to_bits() == glyph_px.to_bits() {
                width += kern(face, previous_id, id, glyph_px);
            }
        }
        width += advance(face, id, glyph_px);
        previous = Some((face, id, glyph_px));
    }
    width
}

fn is_cjk(ch: char) -> bool {
    matches!(ch as u32,
        0x3400..=0x4dbf | 0x4e00..=0x9fff | 0x3000..=0x303f | 0xff00..=0xffef)
}

fn tokens(text: &str) -> Vec<String> {
    let mut result = Vec::new();
    let mut latin_token = String::new();
    let flush = |token: &mut String, output: &mut Vec<String>| {
        if !token.is_empty() {
            output.push(std::mem::take(token));
        }
    };
    for ch in text.chars() {
        if ch.is_whitespace() {
            flush(&mut latin_token, &mut result);
            if result.last().map(String::as_str) != Some(" ") {
                result.push(" ".into());
            }
        } else if is_cjk(ch) {
            flush(&mut latin_token, &mut result);
            result.push(ch.to_string());
        } else {
            latin_token.push(ch);
        }
    }
    flush(&mut latin_token, &mut result);
    result
}

pub fn wrap(text: &str, px: f32, max_width: f32) -> Vec<String> {
    let mut lines = Vec::new();
    let mut line = String::new();
    for token in tokens(text.trim()) {
        let candidate = format!("{line}{token}");
        if !line.is_empty() && measure(&candidate, px) > max_width {
            let finished = line.trim_end();
            if !finished.is_empty() {
                lines.push(finished.to_string());
            }
            line.clear();
        }
        if measure(&token, px) <= max_width {
            if line.is_empty() && token == " " {
                continue;
            }
            line.push_str(&token);
            continue;
        }
        for ch in token.chars() {
            let candidate = format!("{line}{ch}");
            if !line.is_empty() && measure(&candidate, px) > max_width {
                lines.push(line.trim_end().to_string());
                line.clear();
            }
            line.push(ch);
        }
    }
    if !line.trim().is_empty() {
        lines.push(line.trim_end().to_string());
    }
    lines
}

struct PositionedGlyph {
    face: Face,
    glyph: Glyph,
}

pub struct RasterLine {
    pub width: usize,
    pub height: usize,
    pub pixels: Vec<Vec<bool>>,
}

pub fn rasterize_line(text: &str, px: f32) -> RasterLine {
    let mut caret = RASTER_PAD;
    let mut ascent: f32 = 0.0;
    let mut descent: f32 = 0.0;
    let mut specifications = Vec::new();
    let mut previous: Option<(Face, GlyphId, f32)> = None;

    for ch in text.chars() {
        let (face, id, glyph_px) = face_for(ch, px);
        if let Some((previous_face, previous_id, previous_px)) = previous {
            if previous_face == face && previous_px.to_bits() == glyph_px.to_bits() {
                caret += kern(face, previous_id, id, glyph_px);
            }
        }
        let (face_ascent, face_descent) = metrics(face, glyph_px);
        ascent = ascent.max(face_ascent);
        descent = descent.min(face_descent);
        specifications.push((face, id, glyph_px, caret));
        caret += advance(face, id, glyph_px);
        previous = Some((face, id, glyph_px));
    }

    // Font advances are fractional. If an outline is drawn at that fractional
    // x position, identical glyphs acquire different thresholded bitmaps based
    // on the width of the preceding text. That difference is amplified by the
    // skeleton tracer and is especially visible in repeated CJK characters.
    // Quantizing only the raster origin keeps spacing measurements intact while
    // making a glyph's generated Marker paths deterministic.
    let baseline = (RASTER_PAD + ascent).round();
    let width = (caret.ceil() + RASTER_PAD) as usize;
    let height = ((ascent - descent).ceil() + RASTER_PAD * 2.0) as usize;
    let mut glyphs = Vec::with_capacity(specifications.len());
    for (face, id, glyph_px, x) in specifications {
        let x = x.round();
        let mut glyph =
            id.with_scale_and_position(PxScale::from(glyph_px), ab_glyph::point(x, baseline));
        glyph.position.y = baseline;
        glyphs.push(PositionedGlyph { face, glyph });
    }

    let mut pixels = vec![vec![false; width.max(1)]; height.max(1)];
    for positioned in glyphs {
        let outline = match positioned.face {
            Face::Latin => latin().outline_glyph(positioned.glyph),
            Face::CjkHand => cjk_hand().outline_glyph(positioned.glyph),
            Face::CjkFallback => cjk_fallback().outline_glyph(positioned.glyph),
            Face::Symbols => symbols().outline_glyph(positioned.glyph),
        };
        if let Some(outline) = outline {
            let bounds = outline.px_bounds();
            outline.draw(|x, y, coverage| {
                let px_x = bounds.min.x as i32 + x as i32;
                let px_y = bounds.min.y as i32 + y as i32;
                if coverage > 0.5
                    && px_x >= 0
                    && px_y >= 0
                    && (px_x as usize) < width
                    && (px_y as usize) < height
                {
                    pixels[px_y as usize][px_x as usize] = true;
                }
            });
        }
    }
    RasterLine {
        width,
        height,
        pixels,
    }
}

pub fn trace_line(mut line: RasterLine) -> Vec<Vec<(i32, i32)>> {
    skeleton::thin_zhang_suen(&mut line.pixels);
    let mut paths: Vec<Vec<(i32, i32)>> = skeleton::trace_skeleton(&line.pixels)
        .into_iter()
        .map(|path| {
            let mut result = Vec::new();
            for (x, y) in path {
                let point = (x.round() as i32, y.round() as i32);
                if result.last().copied() != Some(point) {
                    result.push(point);
                }
            }
            result
        })
        .filter(|path| path.len() >= 2)
        .collect();
    paths.sort_by_key(|path| path.first().map(|point| point.0).unwrap_or(i32::MAX));
    paths
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn wraps_latin_words_and_chinese_characters() {
        assert!(wrap("Paper Agent writes neatly", 40.0, 170.0).len() >= 2);
        assert!(wrap("這是一段需要自動換行的繁體中文", 48.0, 160.0).len() >= 2);
    }

    #[test]
    fn rasterizes_mixed_language_without_empty_glyphs() {
        let line = rasterize_line("你好, Paper Agent", 52.0);
        assert!(line.pixels.iter().flatten().any(|pixel| *pixel));
        assert!(!trace_line(line).is_empty());
    }

    #[test]
    fn latin_uses_kalam_and_chinese_uses_chen_yu_luoyan() {
        assert_eq!(face_for('A', 52.0).0, Face::Latin);
        assert_eq!(face_for('你', 52.0).0, Face::CjkHand);
        assert_eq!(DEFAULT_CJK_SCALE, 0.70);
    }

    #[test]
    fn common_symbols_use_the_symbol_face_and_unknown_glyphs_keep_question_fallback() {
        for ch in ['♔', '♕', '♖', '♗', '♘', '♙', '♚', '♛', '♜', '♝', '♞', '♟']
        {
            assert_eq!(face_for(ch, 52.0).0, Face::Symbols);
        }
        let (face, id, _) = face_for('\u{10ffff}', 52.0);
        assert_eq!(face, Face::Latin);
        assert_eq!(id, latin().glyph_id('?'));
    }

    #[test]
    fn raster_origins_are_quantized_for_repeatable_glyph_shapes() {
        let (_, id, px) = face_for('月', 112.0);
        let first = RASTER_PAD;
        let second = first + advance(Face::CjkHand, id, px);
        assert_eq!(first.round().fract(), 0.0);
        assert_eq!(second.round().fract(), 0.0);

        let line = rasterize_line("月月", 112.0);
        assert!(line.pixels.iter().flatten().any(|pixel| *pixel));
        assert!(!trace_line(line).is_empty());
    }
}
