# Digital Rain

## Animation notes

Source: https://github.com/carlnewton/digital-rain-analysis

### Standard strings

1. Strings contain random glyphs from a defined set.
1. Strings don't move they just add new glyphs at the bottom for each frame.
1. Strings contain invisible glyphs.
1. Strings don't always start from the top.
    * They are sometimes preceded by invisible glyphs making them look like they start lower down.
1. Some glyphs remain static for three frames, and then change into another glyph.
    * Some strings consist entirely of changing glyphs.

### Highlighted strings

1. Roughly 1 in 5 strings have highlighted glyphs.
    * Only a single glyph of a string is highlighted at a given time, and that glyph is leading glyph of the string.
1. All highlighted glyphs will stammer at the same time.
    * The stammer causes strings with highlighted glyphs to pause behind other strings by a single frame.

### Deletion strings

1. Unlike regular strings, deletion strings can appear over the top of existing strings.
