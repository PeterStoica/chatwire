package message

const (
	maxEmojiRunes = 16
	joiner        = 0x200D
	asEmoji       = 0xFE0F
	asText        = 0xFE0E
	keycap        = 0x20E3
)

var pictographs = [][2]rune{
	{0x00A9, 0x00A9}, {0x00AE, 0x00AE}, {0x203C, 0x203C}, {0x2049, 0x2049}, {0x2122, 0x2122}, {0x2139, 0x2139},
	{0x2194, 0x2199}, {0x21A9, 0x21AA}, {0x231A, 0x231B}, {0x2328, 0x2328}, {0x23CF, 0x23CF}, {0x23E9, 0x23F3},
	{0x23F8, 0x23FA}, {0x24C2, 0x24C2}, {0x25AA, 0x25AB}, {0x25B6, 0x25B6}, {0x25C0, 0x25C0}, {0x25FB, 0x25FE},
	{0x2600, 0x27BF}, {0x2934, 0x2935}, {0x2B05, 0x2B07}, {0x2B1B, 0x2B1C}, {0x2B50, 0x2B50}, {0x2B55, 0x2B55},
	{0x3030, 0x3030}, {0x303D, 0x303D}, {0x3297, 0x3297}, {0x3299, 0x3299}, {0x1F000, 0x1FAFF},
}

func pictograph(r rune) bool {
	for _, span := range pictographs {
		if r >= span[0] && r <= span[1] {
			return true
		}
	}
	return false
}

func modifier(r rune) bool {
	return r == asEmoji || r == asText || r >= 0x1F3FB && r <= 0x1F3FF || r >= 0xE0020 && r <= 0xE007F
}

func regional(r rune) bool {
	return r >= 0x1F1E6 && r <= 0x1F1FF
}

func keycapBase(r rune) bool {
	return r >= '0' && r <= '9' || r == '#' || r == '*'
}

func OneEmoji(s string) bool {
	runes := []rune(s)
	if len(runes) == 0 || len(runes) > maxEmojiRunes {
		return false
	}
	if keycapBase(runes[0]) {
		return len(runes) == 2 && runes[1] == keycap || len(runes) == 3 && runes[1] == asEmoji && runes[2] == keycap
	}
	flags, bases, joined := 0, 0, false
	for i, r := range runes {
		switch {
		case r == joiner:
			if i == 0 || i == len(runes)-1 || joined {
				return false
			}
			joined = true
		case modifier(r):
			if i == 0 {
				return false
			}
		case regional(r):
			flags++
		case pictograph(r):
			if !joined {
				bases++
			}
			joined = false
		default:
			return false
		}
	}
	if flags > 0 {
		return flags == 2 && bases == 0 && len(runes) == 2
	}
	return bases == 1
}
