// SPDX-License-Identifier: MIT
//
// A small display guard built from OpenCC's Apache-2.0 STCharacters table.
// The model is instructed to use Traditional Chinese; this catches unsupported
// Simplified glyphs locally before the handwriting renderer sees them.

use crate::handwriting;
use std::collections::{HashMap, HashSet};
use std::sync::OnceLock;

const ST_CHARACTERS: &str = include_str!("../assets/opencc/STCharacters.txt");

struct ConversionTable {
    replacements: HashMap<char, char>,
    unsupported_simplified: HashSet<char>,
}

fn conversion_table() -> &'static ConversionTable {
    static TABLE: OnceLock<ConversionTable> = OnceLock::new();
    TABLE.get_or_init(|| {
        let mut replacements = HashMap::new();
        let mut unsupported_simplified = HashSet::new();
        for line in ST_CHARACTERS.lines() {
            let Some((source, alternatives)) = line.split_once('\t') else {
                continue;
            };
            let mut source_chars = source.chars();
            let Some(source_char) = source_chars.next() else {
                continue;
            };
            if source_chars.next().is_some() {
                continue;
            }
            let replacement = alternatives
                .split_whitespace()
                .filter_map(|candidate| candidate.chars().next())
                .find(|candidate| handwriting::supports_cjk_char(*candidate));
            if let Some(replacement) = replacement {
                replacements.insert(source_char, replacement);
                if !handwriting::supports_cjk_char(source_char) {
                    unsupported_simplified.insert(source_char);
                }
            }
        }
        ConversionTable {
            replacements,
            unsupported_simplified,
        }
    })
}

pub fn for_display(input: &str) -> String {
    let table = conversion_table();
    if !input
        .chars()
        .any(|ch| table.unsupported_simplified.contains(&ch))
    {
        return input.to_owned();
    }
    input
        .chars()
        .map(|ch| table.replacements.get(&ch).copied().unwrap_or(ch))
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn converts_simplified_output_when_a_marker_is_present() {
        assert_eq!(for_display("很高兴见到你"), "很高興見到你");
    }

    #[test]
    fn leaves_supported_traditional_text_unchanged() {
        let text = "台北的紙上助理";
        assert_eq!(for_display(text), text);
    }
}
