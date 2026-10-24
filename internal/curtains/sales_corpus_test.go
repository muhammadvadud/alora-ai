package curtains

import (
	"encoding/json"
	"os"
	"testing"
)

func TestSalesCorpus_HeldOutQuestions(t *testing.T) {
	data, err := os.ReadFile("../../../backend/apps/knowledge/data/sales_v1.json")
	if os.IsNotExist(err) {
		t.Skip("versiyali backend bilim paketi yonma-yon checkout talab qiladi")
	}
	if err != nil {
		t.Fatal(err)
	}
	var pack struct {
		Rules []struct {
			Key            string         `json:"key"`
			Kind           string         `json:"kind"`
			Condition      map[string]any `json:"condition"`
			Recommendation string         `json:"recommendation"`
			Priority       int            `json:"priority"`
		} `json:"rules"`
	}
	if err := json.Unmarshal(data, &pack); err != nil {
		t.Fatal(err)
	}
	rules := make([]KnowledgeRule, 0, len(pack.Rules))
	for _, r := range pack.Rules {
		rules = append(rules, KnowledgeRule{ID: r.Key, Kind: r.Kind, Condition: r.Condition, Recommendation: r.Recommendation, Priority: r.Priority})
	}
	cases := []struct{ query, want string }{
		{"Baxmalni mashinada yuvsam bo'ladimi?", "care-label-first"},
		{"Бархат можно стирать в машинке?", "care-label-first"},
		{"Тул кечаси чироқ ёнганда кўчадан кўринадими?", "sheer-no-privacy-guarantee"},
		{"Тюль скроет комнату ночью?", "sheer-no-privacy-guarantee"},
		{"Astar Soft qoraytiradigan astarmi?", "lining-not-all-blackout"},
		{"Grafitning rangi to'q bo'lsa yorug'lik o'tmaydimi?", "dark-color-not-opacity"},
		{"Bolam shnurni tortib o'ynaydi xavfsizmi?", "kids-cordless"},
		{"Что безопаснее в детской со шнуром?", "kids-cordless"},
		{"Plita yoniga uzun parda osaymi?", "safety-fire-distance"},
		{"Minimal xonaga lambrekan shartmi?", "minimal-no-lambrequin"},
		{"Kelin kuyovga to'yxonaga o'xshamaydigan parda", "wedding-is-couple-room"},
		{"Rulon eni uch metr bo'lsa balandligi uch metrga yetadimi?", "fabric-width-limit"},
		{"Yirik gulli naqsh rapportiga qo'shimcha mato kerakmi?", "pattern-repeat"},
		{"Balkon eshigi ochilishi qiyin bo'lmaydimi?", "balcony-operation"},
		{"Radiator ustiga tekkizib tushirsak bo'ladimi?", "radiator-clearance"},
		{"Yuvganda qisqarish foizi qancha?", "care-shrinkage"},
		{"Milano Italiyadan kelganmi?", "local-supplier-origin"},
		{"Metri deganingiz pogon metrmi kvadratmi?", "local-meter-sqm"},
		{"Tulim bor yangisini olmayman", "already-has-tul"},
		{"Karnizim bor almashtirmoqchi emasman", "existing-cornice"},
		{"Ertaga to'yga tikib berasizlarmi?", "urgent-deadline"},
		{"Boshqa do'konda arzon ekan", "compare-other-shop"},
		{"Qimmat, lekin kunduz uxlash uchun blackout kerak", "budget-preserve-function"},
		{"Maktab uchun yonmaydigan FR sertifikati bormi?", "fireproof-no-claim"},
		{"Allergiyaga qarshi chang tutmaydimi?", "dust-allergy-claims"},
		{"Wave eski relsimga mos tushadimi?", "wave-track-compatibility"},
		{"Lyuversni oddiy relsga ilamizmi?", "not-all-rods-for-eyelets"},
		{"Avans aytsangiz to'lov qilingan hisoblanadimi?", "quote-is-not-payment"},
		{"Qaytarish va kafolat shartlari qanday?", "returns-and-guarantee"},
		{"Viloyatga ustaning chiqishi bepulmi?", "local-travel-and-install"},
	}
	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			found := KnowledgeSearch(rules, SessionState{}, "general", tc.query)
			if len(found) > 6 {
				found = found[:6]
			}
			for _, r := range found {
				if r.ID == tc.want {
					return
				}
			}
			ids := make([]string, 0, len(found))
			for _, r := range found {
				ids = append(ids, r.ID)
			}
			t.Errorf("kerakli bilim %s top-6 ichida yo'q: %v", tc.want, ids)
		})
	}
}
