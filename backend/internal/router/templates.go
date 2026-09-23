package router

import (
	"fmt"
	"sort"
	"strings"

	"hackathon/backend/internal/catalog"
	"hackathon/backend/internal/lang"
)

// Templates produce deterministic spoken replies from the catalog and the
// knowledge base. They power the keyless router and the fast path.
type Templates struct {
	cat   *catalog.Catalog
	names func(id, language string) string
}

func NewTemplates(cat *catalog.Catalog, names func(id, language string) string) *Templates {
	return &Templates{cat: cat, names: names}
}

var cityWords = map[string]string{
	"алматы": "Almaty", "алмате": "Almaty", "алматыда": "Almaty", "астан": "Astana", "нур-султан": "Astana", "шымкент": "Shymkent", "караганд": "Karaganda", "қарағанд": "Karaganda",
	"актобе": "Aktobe", "ақтөбе": "Aktobe", "атырау": "Atyrau", "павлодар": "Pavlodar", "усть-каменогорск": "Oskemen", "өскемен": "Oskemen", "оскемен": "Oskemen",
}

// DetectCity finds a known city in the utterance.
func DetectCity(text string) string {
	norm := lang.Normalize(text)
	for w, city := range cityWords {
		if strings.Contains(norm, w) {
			return city
		}
	}
	return ""
}

func localizeCity(city, language string) string {
	m := map[string]map[string]string{
		"Almaty": {"ru": "Алматы", "kk": "Алматы"}, "Astana": {"ru": "Астана", "kk": "Астана"}, "Shymkent": {"ru": "Шымкент", "kk": "Шымкент"},
		"Karaganda": {"ru": "Караганда", "kk": "Қарағанды"}, "Aktobe": {"ru": "Актобе", "kk": "Ақтөбе"}, "Atyrau": {"ru": "Атырау", "kk": "Атырау"},
		"Pavlodar": {"ru": "Павлодар", "kk": "Павлодар"}, "Oskemen": {"ru": "Усть-Каменогорск", "kk": "Өскемен"},
	}
	if v, ok := m[city]; ok {
		return v[language]
	}
	return city
}

// Opening returns the scenario's opening line in the language.
func (t *Templates) Opening(id, language string) string {
	if sc := t.cat.Scenario(id); sc != nil {
		if r, ok := sc.Responses[language]; ok && r.Opening != "" {
			return r.Opening
		}
		if r, ok := sc.Responses["ru"]; ok {
			return r.Opening
		}
	}
	if si := t.cat.System(id); si != nil {
		if r, ok := si.Response[language]; ok {
			return r
		}
		return si.Response["ru"]
	}
	return ""
}

// Clarify renders the SYS_UNCLEAR question with two options.
func (t *Templates) Clarify(language, a, b string) string {
	si := t.cat.System("SYS_UNCLEAR")
	tpl := "Уточните, пожалуйста: вы хотите {option_a} или {option_b}?"
	if si != nil {
		if r, ok := si.Response[language]; ok {
			tpl = r
		}
	}
	if a == "" {
		if language == "kk" {
			return "Нақтылап жіберіңізші: сізге қандай мәселе бойынша көмек керек?"
		}
		return "Уточните, пожалуйста, с чем именно помочь: со страховкой авто, здоровья, жилья или с выплатой?"
	}
	if b == "" {
		b = map[string]string{"ru": "что-то другое", "kk": "басқа нәрсе"}[language]
	}
	return strings.NewReplacer("{option_a}", t.names(a, language), "{option_b}", t.names(b, language)).Replace(tpl)
}

// Info builds a knowledge-base answer for informational scenarios; it falls
// back to the scenario's opening line.
func (t *Templates) Info(id, language, utterance string) string {
	city := DetectCity(utterance)
	switch id {
	case "SC33":
		if v, ok := t.cat.KBSection("offices"); ok {
			for _, o := range v.([]any) {
				m := o.(map[string]any)
				if city != "" && strings.EqualFold(fmt.Sprint(m["city"]), city) {
					if language == "kk" {
						return fmt.Sprintf("%s кеңсесі: %s, жұмыс уақыты %s. Мекенжайды SMS-пен жіберейін бе?", localizeCity(city, "kk"), m["address"], m["hours"])
					}
					return fmt.Sprintf("Офис в городе %s: %s, часы работы %s. Отправить адрес в SMS?", localizeCity(city, "ru"), m["address"], m["hours"])
				}
			}
			if city != "" {
				if language == "kk" {
					return fmt.Sprintf("%s қаласында кеңсе жоқ, бірақ полисті қосымшада онлайн рәсімдеуге болады. Ең жақын кеңселер: Алматы, Астана, Шымкент.", localizeCity(city, "kk"))
				}
				return fmt.Sprintf("В городе %s офиса нет, но всё можно оформить онлайн в приложении. Ближайшие офисы: Алматы, Астана, Шымкент.", localizeCity(city, "ru"))
			}
		}
	case "SC23":
		if v, ok := t.cat.KBSection("clinics"); ok {
			names := []string{}
			for _, c := range v.([]any) {
				m := c.(map[string]any)
				if city == "" || strings.EqualFold(fmt.Sprint(m["city"]), city) {
					names = append(names, fmt.Sprintf("%s (%s)", m["name"], m["address"]))
				}
			}
			if city != "" && len(names) > 0 {
				if language == "kk" {
					return fmt.Sprintf("%s қаласында ДМС бойынша қабылдайтын емханалар: %s. Тізімді SMS-пен жіберейін бе?", localizeCity(city, "kk"), strings.Join(names, ", "))
				}
				return fmt.Sprintf("В городе %s по ДМС принимают: %s. Отправить список в SMS?", localizeCity(city, "ru"), strings.Join(names, ", "))
			}
			if city != "" {
				if language == "kk" {
					return fmt.Sprintf("%s қаласында серіктес емхана жоқ. Ең жақын емханалар Алматы, Астана, Шымкент және Қарағандыда.", localizeCity(city, "kk"))
				}
				return fmt.Sprintf("В городе %s партнёрских клиник пока нет. Ближайшие — в Алматы, Астане, Шымкенте и Караганде.", localizeCity(city, "ru"))
			}
		}
	case "SC31":
		if language == "kk" {
			return "Төлеуге болады: қосымшада немесе сайтта банк картасымен, SMS-тегі сілтеме арқылы, кеңседегі терминалда. КАСКО-ны 2 немесе 4 төлемге бөлуге болады, ОГПО мен сапар сақтандыруы толық төленеді. Қолма-қол ақша қабылданбайды."
		}
		return "Оплатить можно картой в приложении или на сайте, по ссылке из SMS, а также через терминал в офисе. КАСКО можно разбить на 2 или 4 платежа без переплаты, ОГПО и страховка для поездок оплачиваются полностью. Наличные не принимаем."
	case "SC34":
		if language == "kk" {
			return "Қосымшаға телефон нөмірі және SMS-коды арқылы кіресіз. Код келмесе, нөмірді тексеріп, 60 секундтан кейін жаңа код сұратыңыз. Көмектеспесе, операторға қосамын."
		}
		return "Вход в приложение — по номеру телефона и коду из SMS. Если код не приходит, проверьте номер и через 60 секунд запросите новый. Если не поможет, соединю с оператором."
	case "SC18":
		prod := productFromText(utterance)
		key := map[string]string{"casco": "casco", "ogpo": "ogpo_victim", "property": "property", "accident": "accident", "travel": "travel"}[prod]
		if key != "" {
			if v, ok := t.cat.KBSection("claims.documents." + key); ok {
				docs := []string{}
				for _, d := range v.([]any) {
					docs = append(docs, translateDoc(fmt.Sprint(d), language))
				}
				if language == "kk" {
					return fmt.Sprintf("Қажетті құжаттар: %s. Оларды қосымшада немесе claims@saqta-insurance.example поштасына жіберуге болады.", strings.Join(docs, ", "))
				}
				return fmt.Sprintf("Понадобятся: %s. Загрузить их можно в приложении или отправить на claims@saqta-insurance.example.", strings.Join(docs, ", "))
			}
		}
	}
	return t.Opening(id, language)
}

func productFromText(text string) string {
	n := lang.Normalize(text)
	switch {
	case strings.Contains(n, "каско"):
		return "casco"
	case strings.Contains(n, "затоп") || strings.Contains(n, "квартир") || strings.Contains(n, "пожар") || strings.Contains(n, "пәтер") || strings.Contains(n, "үй") || strings.Contains(n, "су басып") || strings.Contains(n, "өрт"):
		return "property"
	case strings.Contains(n, "дтп") || strings.Contains(n, "авари") || strings.Contains(n, "жол апат") || strings.Contains(n, "огпо") || strings.Contains(n, "виновник"):
		return "ogpo"
	case strings.Contains(n, "травм") || strings.Contains(n, "несчаст") || strings.Contains(n, "жазатайым") || strings.Contains(n, "жарақат"):
		return "accident"
	case strings.Contains(n, "за границ") || strings.Contains(n, "поездк") || strings.Contains(n, "шетел") || strings.Contains(n, "сапар"):
		return "travel"
	}
	return ""
}

var docTranslations = map[string]map[string]string{
	"ID card":                                   {"ru": "удостоверение личности", "kk": "жеке куәлік"},
	"Driving licence":                           {"ru": "водительское удостоверение", "kk": "жүргізуші куәлігі"},
	"Vehicle registration certificate":          {"ru": "техпаспорт", "kk": "техпаспорт"},
	"Road accident documents from the police":   {"ru": "документы о ДТП из полиции", "kk": "полициядан жол апаты құжаттары"},
	"Bank details":                              {"ru": "банковские реквизиты", "kk": "банк деректемелері"},
	"Photos of the damage":                      {"ru": "фото повреждений", "kk": "зақым фотосуреттері"},
	"Police documents (if police was involved)": {"ru": "документы из полиции, если она вызывалась", "kk": "полиция құжаттары, егер шақырылса"},
	"Policy number":                             {"ru": "номер полиса", "kk": "полис нөмірі"},
	"Act from the building management company (for water damage) or fire service report (for fire)": {"ru": "акт от управляющей компании при заливе или справка пожарной службы при пожаре", "kk": "су басқанда басқарушы компанияның актісі немесе өрт кезінде өрт қызметінің анықтамасы"},
	"Medical certificate from the trauma centre or hospital":                                        {"ru": "справка из травмпункта или больницы", "kk": "травмпункт немесе аурухана анықтамасы"},
	"Medical documents from abroad":                                                                 {"ru": "медицинские документы из-за границы", "kk": "шетелдік медициналық құжаттар"},
	"Receipts (only for expenses agreed with assistance)":                                           {"ru": "чеки по расходам, согласованным с ассистансом", "kk": "ассистанспен келісілген шығындар чектері"},
}

func translateDoc(d, language string) string {
	if m, ok := docTranslations[d]; ok {
		if v, ok := m[language]; ok {
			return v
		}
	}
	return d
}

// SortedIDs is a helper for deterministic output.
func SortedIDs(m map[string]float64) []string {
	ids := make([]string, 0, len(m))
	for k := range m {
		ids = append(ids, k)
	}
	sort.Slice(ids, func(i, j int) bool { return m[ids[i]] > m[ids[j]] })
	return ids
}
