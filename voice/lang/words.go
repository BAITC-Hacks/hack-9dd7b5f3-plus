package lang

// kazakhOnlyLetters are Cyrillic letters that exist only in the Kazakh
// alphabet and never in Russian; their presence proves a word is Kazakh.
var kazakhOnlyLetters = map[rune]bool{
	'ә': true, 'ғ': true, 'қ': true, 'ң': true, 'ө': true,
	'ұ': true, 'ү': true, 'һ': true, 'і': true,
}

// kazakhWordsRU is a built-in list of frequent Kazakh words spelled using
// only Russian-alphabet Cyrillic letters, so kazakhOnlyLetters can't spot
// them. Words that double as common Russian words (не, да, бар, алло, аз,
// ...) are deliberately left out to avoid false positives.
var kazakhWordsRU = map[string]bool{
	"мен": true, "сен": true, "ол": true, "біз": true, "сіз": true,
	"олар": true, "жоқ": true, "иә": true, "жарайды": true, "рахмет": true,
	"керек": true, "бе": true, "ма": true, "ме": true, "ба": true,
	"па": true, "пе": true, "кеше": true, "ертең": true, "енді": true,
	"сосын": true, "және": true, "немесе": true, "деген": true, "екен": true,
	"еді": true, "емес": true, "болады": true, "болды": true, "бола": true,
	"жатыр": true, "жатырмын": true, "келеді": true, "келді": true, "кетті": true,
	"алдым": true, "бердім": true, "маған": true, "саған": true, "мына": true,
	"сол": true, "осы": true, "тез": true, "жақсы": true, "сәлем": true,
	"сәлеметсіз": true, "саламатсыз": true, "мархабат": true, "кешіріңіз": true,
	"айтыңызшы": true, "ия": true,
}

// kazakhSuffixes are person/number and possessive-case endings that mark
// a long enough word as Kazakh even when every letter also exists in
// Russian.
var kazakhSuffixes = []string{
	"мын", "мін", "пын", "пін", "бын", "бін",
	"мыз", "міз", "сыз", "сіз", "ңыз", "ңіз",
	"дым", "дім", "тым", "тім",
}

// instrumentalSuffixes are the Kazakh instrumental case endings.
var instrumentalSuffixes = []string{"мен", "пен", "бен"}

// instrumentalExceptions are Russian words that end like the Kazakh
// instrumental case but are not Kazakh.
var instrumentalExceptions = map[string]bool{
	"экзамен": true, "феномен": true, "абдомен": true,
}

// ignoredWords are single-letter Russian words too ambiguous to count
// towards either language when analyzing a whole utterance.
var ignoredWords = map[string]bool{
	"я": true, "а": true, "и": true, "в": true,
	"с": true, "у": true, "к": true, "о": true,
}

// kkTriggers are phrases that ask the robot to switch to Kazakh.
var kkTriggers = []string{
	"қазақша", "казахша", "по-казахски", "по казахски", "на казахском", "қазақ тілінде",
}

// ruTriggers are phrases that ask the robot to switch to Russian.
var ruTriggers = []string{
	"орысша", "по-русски", "по русски", "на русском", "орыс тілінде",
}
