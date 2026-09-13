package voice

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	rePhone1     = regexp.MustCompile(`(?i)\+998[\s\-]*\d(?:[\s\-]*\d){8}`)
	rePhone2     = regexp.MustCompile(`(?i)\b(?:90|91|93|94|95|97|98|99|33|88|77|55|71)[\s\-]*\d{3}[\s\-]*\d{2}[\s\-]*\d{2}\b`)
	reSpaces     = regexp.MustCompile(`(\d)\s+(\d)`)
	reKoef       = regexp.MustCompile(`(?i)\bK\s*=\s*`)
	reNumUnit    = regexp.MustCompile(`(?i)(koeffitsient\s+)?(\d+(?:\.\d+)?)((\s*)([a-zа-яё²%'\.,;\)0-9]+))?`)
	reMultispace = regexp.MustCompile(`\s+`)
)

// NormalizeForTTS matnni o'qiladigan so'zlarga aylantiradi (faqat TTS kirishida).
// lang: "uz" yoki "ru".
func NormalizeForTTS(text string, lang string) string {
	// 1. Ketma-ket bo'shliqlar va non-breaking space (nbsp) ni tozalash
	text = strings.ReplaceAll(text, "\u00A0", " ")

	// 2. Telefon raqamlarini maskalash (PII o'qilmaydi)
	text = rePhone1.ReplaceAllString(text, "telefon raqami")
	text = rePhone2.ReplaceAllString(text, "telefon raqami")

	// 3. Sonlar orasidagi bo'shliqlarni birlashtirish (pul: 2 500 000 -> 2500000)
	for {
		newText := reSpaces.ReplaceAllString(text, "${1}${2}")
		if newText == text {
			break
		}
		text = newText
	}

	// 4. K= qisqartmasini kengaytirish
	text = reKoef.ReplaceAllString(text, "koeffitsient ")

	// 5. Sonlar va o'lchov birliklarini tarjima qilish
	text = reNumUnit.ReplaceAllStringFunc(text, func(match string) string {
		groups := reNumUnit.FindStringSubmatch(match)
		koef := strings.TrimSpace(groups[1])
		numStr := groups[2]
		rawUnitGroup := groups[3]
		potentialUnitRaw := groups[5]
		potentialUnit := strings.ToLower(potentialUnitRaw)

		validUnits := map[string]bool{
			"m²": true, "m2": true, "kv.m": true, "m": true, "sm": true,
			"%": true, "mln": true, "ming": true, "so'm": true, "сум": true,
		}

		// Tinish belgilarini ajratish
		punct := ""
	punctLoop:
		for potentialUnit != "" && potentialUnit != "kv.m" {
			switch potentialUnit[len(potentialUnit)-1] {
			case '.':
				punct = "." + punct
				potentialUnit = potentialUnit[:len(potentialUnit)-1]
			case ',':
				punct = "," + punct
				potentialUnit = potentialUnit[:len(potentialUnit)-1]
			case ';':
				punct = ";" + punct
				potentialUnit = potentialUnit[:len(potentialUnit)-1]
			case ')':
				punct = ")" + punct
				potentialUnit = potentialUnit[:len(potentialUnit)-1]
			default:
				break punctLoop
			}
		}

		isUnit := validUnits[potentialUnit]

		// Sonni ajratish (butun va kasr qismi)
		var isDecimal bool
		var intPart int64
		var fracWordsUz, fracWordsRu []string

		if strings.Contains(numStr, ".") {
			isDecimal = true
			parts := strings.Split(numStr, ".")
			intPart, _ = strconv.ParseInt(parts[0], 10, 64)
			for _, ch := range parts[1] {
				digit, _ := strconv.ParseInt(string(ch), 10, 64)
				fracWordsUz = append(fracWordsUz, NumberToWordsUz(digit))
				fracWordsRu = append(fracWordsRu, NumberToWordsRu(digit, "m"))
			}
		} else {
			intPart, _ = strconv.ParseInt(numStr, 10, 64)
		}

		spokenUz, spokenRu := "", ""
		if isDecimal {
			spokenUz = NumberToWordsUz(intPart) + " butun " + strings.Join(fracWordsUz, " ")
			intRu := NumberToWordsRu(intPart, "f")
			rem10 := intPart % 10
			rem100 := intPart % 100
			if rem10 == 1 && rem100 != 11 {
				intRu += " целая"
			} else {
				intRu += " целых"
			}
			spokenRu = intRu + " " + strings.Join(fracWordsRu, " ")
		} else {
			spokenUz = NumberToWordsUz(intPart)
			spokenRu = NumberToWordsRu(intPart, "m")
		}

		prefixUz, prefixRu := "", ""
		if koef != "" {
			prefixUz = "koeffitsient "
			prefixRu = "коэффициент "
		}

		var result string
		if lang == "ru" {
			result = prefixRu + spokenRu
			if isUnit {
				uRu := getUnitRu(potentialUnit, isDecimal, intPart)
				if uRu != "" {
					result += " " + uRu + punct
				} else {
					result += punct
				}
			} else {
				result += rawUnitGroup
			}
		} else {
			result = prefixUz + spokenUz
			if isUnit {
				uUz := getUnitUz(potentialUnit)
				if uUz != "" {
					result += " " + uUz + punct
				} else {
					result += punct
				}
			} else {
				result += rawUnitGroup
			}
		}
		return result
	})

	// 6. Qolgan ortiqcha bo'shliqlarni tozalash
	text = reMultispace.ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)

	return text
}

// NumberToWordsUz o'zbek tilida sonni so'zga aylantiradi.
func NumberToWordsUz(n int64) string {
	if n == 0 {
		return "nol"
	}
	if n < 0 {
		return "minus " + NumberToWordsUz(-n)
	}

	var parts []string

	b := (n / 1000000000) % 1000
	if b > 0 {
		parts = append(parts, formatUzGroup(b)+" milliard")
	}

	m := (n / 1000000) % 1000
	if m > 0 {
		parts = append(parts, formatUzGroup(m)+" million")
	}

	th := (n / 1000) % 1000
	if th > 0 {
		if th == 1 {
			parts = append(parts, "ming")
		} else {
			parts = append(parts, formatUzGroup(th)+" ming")
		}
	}

	u := n % 1000
	if u > 0 {
		parts = append(parts, formatUzGroup(u))
	}

	return strings.Join(parts, " ")
}

func formatUzGroup(n int64) string {
	var words []string

	h := n / 100
	if h > 0 {
		if h == 1 {
			words = append(words, "yuz")
		} else {
			units := []string{"", "bir", "ikki", "uch", "to'rt", "besh", "olti", "yetti", "sakkiz", "to'qqiz"}
			words = append(words, units[h]+" yuz")
		}
	}

	rem := n % 100
	t := rem / 10
	if t > 0 {
		tens := []string{"", "o'n", "yigirma", "o'ttiz", "qirq", "ellik", "oltmish", "yetmish", "sakson", "to'qson"}
		words = append(words, tens[t])
	}
	u := rem % 10
	if u > 0 {
		units := []string{"", "bir", "ikki", "uch", "to'rt", "besh", "olti", "yetti", "sakkiz", "to'qqiz"}
		words = append(words, units[u])
	}

	return strings.Join(words, " ")
}

// NumberToWordsRu rus tilida sonni so'zga aylantiradi.
func NumberToWordsRu(n int64, gender string) string {
	if n == 0 {
		return "ноль"
	}
	if n < 0 {
		return "минус " + NumberToWordsRu(-n, gender)
	}

	var parts []string

	b := (n / 1000000000) % 1000
	if b > 0 {
		parts = append(parts, formatRuGroup(b, "m", "миллиард", "миллиарда", "миллиардов"))
	}

	m := (n / 1000000) % 1000
	if m > 0 {
		parts = append(parts, formatRuGroup(m, "m", "миллион", "миллиона", "миллионов"))
	}

	th := (n / 1000) % 1000
	if th > 0 {
		parts = append(parts, formatRuGroup(th, "f", "тысяча", "тысячи", "тысяч"))
	}

	u := n % 1000
	if u > 0 || len(parts) == 0 {
		if u > 0 {
			parts = append(parts, formatRuGroup(u, gender, "", "", ""))
		}
	}

	return strings.Join(parts, " ")
}

func formatRuGroup(n int64, gender string, form1, form2, form5 string) string {
	var words []string

	h := n / 100
	if h > 0 {
		hundreds := []string{"", "сто", "двести", "триста", "четыреста", "пятьсот", "шестьсот", "семьсот", "восемьсот", "девятьсот"}
		words = append(words, hundreds[h])
	}

	rem := n % 100
	if rem >= 11 && rem <= 19 {
		teens := []string{"", "одиннадцать", "двенадцать", "тринадцать", "четырнадцать", "пятнадцать", "шестнадцать", "семнадцать", "восемнадцать", "девятнадцать"}
		words = append(words, teens[rem-10])
	} else {
		t := rem / 10
		if t > 0 {
			tens := []string{"", "десять", "двадцать", "тридцать", "сорок", "пятьдесят", "шестьдесят", "семьдесят", "восемьдесят", "девяносто"}
			words = append(words, tens[t])
		}
		u := rem % 10
		if u > 0 {
			switch u {
			case 1:
				switch gender {
				case "f":
					words = append(words, "одна")
				case "n":
					words = append(words, "одно")
				default:
					words = append(words, "один")
				}
			case 2:
				switch gender {
				case "f":
					words = append(words, "две")
				default:
					words = append(words, "два")
				}
			default:
				units := []string{"", "", "", "три", "четыре", "пять", "шесть", "семь", "восемь", "девять"}
				words = append(words, units[u])
			}
		}
	}

	if form1 != "" {
		rem100 := n % 100
		rem10 := n % 10
		switch {
		case rem100 >= 11 && rem100 <= 14:
			words = append(words, form5)
		case rem10 == 1:
			words = append(words, form1)
		case rem10 >= 2 && rem10 <= 4:
			words = append(words, form2)
		default:
			words = append(words, form5)
		}
	}

	return strings.Join(words, " ")
}

func getUnitUz(unit string) string {
	switch unit {
	case "m²", "m2", "kv.m":
		return "kvadrat metr"
	case "m":
		return "metr"
	case "sm":
		return "santimetr"
	case "%":
		return "foiz"
	case "mln":
		return "million"
	case "ming":
		return "ming"
	case "so'm", "сум":
		return "so'm"
	}
	return ""
}

func getUnitRu(unit string, isDecimal bool, intPart int64) string {
	if unit == "so'm" || unit == "сум" {
		return "сум"
	}
	if unit == "ming" {
		return "ming"
	}

	form1, form2, form5 := "", "", ""
	switch unit {
	case "m²", "m2", "kv.m":
		form1, form2, form5 = "квадратный метр", "квадратных метра", "квадратных метров"
	case "m":
		form1, form2, form5 = "метр", "метра", "метров"
	case "sm":
		form1, form2, form5 = "сантиметр", "сантиметра", "сантиметров"
	case "%":
		form1, form2, form5 = "процент", "процента", "процентов"
	case "mln":
		form1, form2, form5 = "миллион", "миллиона", "миллионов"
	default:
		return ""
	}

	if isDecimal {
		return form2
	}
	return declineRu(intPart, form1, form2, form5)
}

func declineRu(n int64, form1, form2, form5 string) string {
	rem100 := n % 100
	rem10 := n % 10
	if rem100 >= 11 && rem100 <= 14 {
		return form5
	}
	if rem10 == 1 {
		return form1
	}
	if rem10 >= 2 && rem10 <= 4 {
		return form2
	}
	return form5
}
