package message

const (
	maxEmojiRunes = 16
	joiner        = 0x200D
	asEmoji       = 0xFE0F
	asText        = 0xFE0E
	keycap        = 0x20E3
)

func pictograph(r rune) bool {
	return r == 0x00A9 || r == 0x00AE || r == 0x203C || r == 0x2049 || r == 0x2122 || r == 0x2139 ||
		r >= 0x2194 && r <= 0x2199 || r >= 0x21A9 && r <= 0x21AA || r >= 0x231A && r <= 0x231B || r == 0x2328 ||
		r == 0x23CF || r >= 0x23E9 && r <= 0x23F3 || r >= 0x23F8 && r <= 0x23FA || r == 0x24C2 ||
		r >= 0x25AA && r <= 0x25AB || r == 0x25B6 || r == 0x25C0 || r >= 0x25FB && r <= 0x25FE ||
		r >= 0x2600 && r <= 0x27BF || r >= 0x2934 && r <= 0x2935 || r >= 0x2B05 && r <= 0x2B07 ||
		r >= 0x2B1B && r <= 0x2B1C || r == 0x2B50 || r == 0x2B55 || r == 0x3030 || r == 0x303D || r == 0x3297 ||
		r == 0x3299 || r >= 0x1F000 && r <= 0x1FAFF
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
