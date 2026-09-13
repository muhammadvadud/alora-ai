package voice

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

var abbreviations = []string{
	"sm.", "mln.", "ming.", "m.", "dona.", "so'm.",
}

// SentenceSplitter matn oqimini jumlalarga ajratuvchi yordamchi tuzilma.
type SentenceSplitter struct {
	MaxChars int
	buffer   strings.Builder
}

func (s *SentenceSplitter) init() {
	if s.MaxChars == 0 {
		s.MaxChars = 120
	}
}

// Push oqimdan kelgan matn bo'laklarini yig'adi va to'liq jumlalarni qaytaradi.
// Chegara: . ! ? va undan keyin bo'shliq/qator oxiri.
// Qisqartmalar (sm., mln. va hk) va raqam ichidagi nuqta chegara hisoblanmaydi.
func (s *SentenceSplitter) Push(chunk string) []string {
	s.init()
	s.buffer.WriteString(chunk)

	var sentences []string

	for {
		text := s.buffer.String()
		if text == "" {
			break
		}

		splitIndex := -1

		// 1. Jumla chegarasini izlash
		for i := 0; i < len(text); i++ {
			c := text[i]
			if c == '.' || c == '!' || c == '?' {
				// Matn oxiriga kelgan bo'lsak, keyingi bo'lakni kutamiz (balki u space emasdir)
				if i+1 >= len(text) {
					continue
				}

				nextRune, _ := utf8.DecodeRuneInString(text[i+1:])
				if unicode.IsSpace(nextRune) {
					isBoundary := true

					if c == '.' {
						// Nuqtadan oldingi so'zni aniqlash
						wordStart := i
						for wordStart > 0 {
							prevRune, size := utf8.DecodeLastRuneInString(text[:wordStart])
							if unicode.IsSpace(prevRune) {
								break
							}
							wordStart -= size
						}
						word := strings.ToLower(text[wordStart : i+1])

						// Qisqartma ekanligini tekshirish
						for _, abbr := range abbreviations {
							if strings.HasSuffix(word, abbr) {
								prefix := strings.TrimSuffix(word, abbr)
								onlyNonLetters := true
								for _, r := range prefix {
									if unicode.IsLetter(r) {
										onlyNonLetters = false
										break
									}
								}
								if onlyNonLetters {
									isBoundary = false
									break
								}
							}
						}
					}

					if isBoundary {
						splitIndex = i + 1
						break
					}
				}
			}
		}

		// 2. MaxChars dan oshganda oxirgi bo'shliqdan kesish
		if splitIndex == -1 && utf8.RuneCountInString(text) > s.MaxChars {
			lastSpace := -1
			runeCount := 0
			for i, r := range text {
				if runeCount > s.MaxChars {
					break
				}
				if unicode.IsSpace(r) {
					lastSpace = i
				}
				runeCount++
			}

			if lastSpace != -1 {
				splitIndex = lastSpace
			}
		}

		// Jumla topilgan bo'lsa uni ajratamiz, yo'qsa kutamiz
		if splitIndex != -1 {
			sentence := strings.TrimSpace(text[:splitIndex])
			if sentence != "" {
				sentences = append(sentences, sentence)
			}
			rem := strings.TrimLeftFunc(text[splitIndex:], unicode.IsSpace)
			s.buffer.Reset()
			s.buffer.WriteString(rem)
		} else {
			break
		}
	}

	return sentences
}

// Flush qolgan matnni qaytaradi va buferni tozalaydi.
func (s *SentenceSplitter) Flush() string {
	res := strings.TrimSpace(s.buffer.String())
	s.buffer.Reset()
	return res
}
