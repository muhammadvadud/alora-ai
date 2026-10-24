package curtains

import "strings"

type knowledgeTerm struct {
	prefix string
	term   string
}

var knowledgeTerms = []knowledgeTerm{
	{"стир", "parvarish"}, {"чист", "parvarish"}, {"ухаж", "parvarish"}, {"уход", "parvarish"},
	{"yuvi", "parvarish"}, {"yuvm", "parvarish"}, {"пости", "parvarish"}, {"tozala", "parvarish"}, {"parvarish", "parvarish"}, {"wash", "parvarish"}, {"care", "parvarish"},
	{"спальн", "yotoqxona"}, {"bedroom", "yotoqxona"}, {"yotoq", "yotoqxona"},
	{"детск", "bolalar"}, {"ребен", "bolalar"}, {"ребён", "bolalar"}, {"children", "bolalar"}, {"kids", "bolalar"}, {"bola", "bolalar"},
	{"гостин", "mehmonxona"}, {"living", "mehmonxona"}, {"mehmon", "mehmonxona"},
	{"кухн", "oshxona"}, {"kitchen", "oshxona"}, {"oshxona", "oshxona"},
	{"офис", "ofis"}, {"office", "ofis"}, {"кабинет", "ofis"},
	{"wedding", "kelinkuyov"}, {"kelin", "kelinkuyov"}, {"невест", "kelinkuyov"},
	{"блэкаут", "blackout"}, {"блекаут", "blackout"}, {"blekaut", "blackout"}, {"blackout", "blackout"},
	{"затемн", "qoraytirish"}, {"темнот", "qoraytirish"}, {"qorong", "qoraytirish"}, {"qoray", "qoraytirish"},
	{"полупрозрач", "semi"}, {"shaffof", "shaffof"}, {"прозрач", "shaffof"},
	{"приват", "maxfiylik"}, {"уедин", "maxfiylik"}, {"privacy", "maxfiylik"}, {"maxfiy", "maxfiylik"},
	{"ноч", "kechasi"}, {"kecha", "kechasi"}, {"kechasi", "kechasi"}, {"night", "kechasi"},
	{"солн", "quyosh"}, {"sunlight", "quyosh"}, {"quyosh", "quyosh"},
	{"дорог", "qimmat"}, {"expensive", "qimmat"}, {"qimmat", "qimmat"},
	{"дешев", "arzon"}, {"дешёв", "arzon"}, {"arzon", "arzon"}, {"бюджет", "budjet"},
	{"скид", "chegirma"}, {"discount", "chegirma"}, {"chegirma", "chegirma"},
	{"безопас", "xavfsizlik"}, {"xavfsiz", "xavfsizlik"}, {"safety", "xavfsizlik"},
	{"шнур", "shnur"}, {"cord", "shnur"}, {"shnur", "shnur"},
	{"карниз", "karniz"}, {"karniz", "karniz"}, {"балкон", "balkon"},
	{"замер", "olchov"}, {"измер", "olchov"}, {"measure", "olchov"}, {"oʼlch", "olchov"},
	{"ширин", "kenglik"}, {"высот", "balandlik"}, {"длин", "uzunlik"},
	{"бархат", "baxmal"}, {"barxat", "baxmal"}, {"velvet", "baxmal"}, {"baxmal", "baxmal"},
	{"вуаль", "vual"}, {"тюль", "tul"}, {"тюля", "tul"}, {"tyul", "tul"},
	{"подклад", "astar"}, {"lining", "astar"}, {"astar", "astar"},
	{"лен", "zigir"}, {"лён", "zigir"}, {"linen", "zigir"}, {"zigʼir", "zigir"},
	{"усад", "qisqarish"}, {"shrink", "qisqarish"}, {"qisqar", "qisqarish"},
	{"аллерг", "allergiya"}, {"allerg", "allergiya"}, {"пыл", "chang"},
	{"состав", "tarkib"}, {"tarkib", "tarkib"}, {"ярлык", "yorliq"}, {"yorliq", "yorliq"},
	{"ламбрек", "lambrekan"}, {"lambrek", "lambrekan"}, {"волна", "tolqin"}, {"wave", "tolqin"},
	{"минимал", "minimal"}, {"minimal", "minimal"}, {"классик", "classic"}, {"klassik", "classic"},
	{"современ", "modern"}, {"zamonaviy", "modern"}, {"прованс", "provence"}, {"provans", "provence"},
	{"восточ", "oriental"}, {"sharqona", "oriental"},
}

var knowledgeCyrillic = strings.NewReplacer(
	"ў", "oʼ", "қ", "q", "ғ", "gʼ", "ҳ", "h", "ё", "yo", "ю", "yu", "я", "ya",
	"а", "a", "б", "b", "в", "v", "г", "g", "д", "d", "е", "e", "ж", "j", "з", "z",
	"и", "i", "й", "y", "к", "k", "л", "l", "м", "m", "н", "n", "о", "o", "п", "p",
	"р", "r", "с", "s", "т", "t", "у", "u", "ф", "f", "х", "x", "ц", "ts", "ч", "ch",
	"ш", "sh", "щ", "sh", "ъ", "", "ь", "", "ы", "i", "э", "e",
)

func canonicalKnowledgeToken(token string) string {
	if translated := matchKnowledgeTerm(token); translated != "" {
		return translated
	}
	token = knowledgeCyrillic.Replace(token)
	if translated := matchKnowledgeTerm(token); translated != "" {
		return translated
	}
	return token
}

func matchKnowledgeTerm(token string) string {
	for _, alias := range knowledgeTerms {
		if token == alias.prefix || countRunes(alias.prefix) >= 4 && strings.HasPrefix(token, alias.prefix) {
			return alias.term
		}
	}
	return ""
}
