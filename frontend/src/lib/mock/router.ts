/**
 * MOCK router — keyless, deterministic, runs in the browser.
 *
 * ⚠️ This is NOT the product router. The product router is an LLM (see docs/SPEC.md);
 * this mock only exists so the UI can be developed and demoed without API keys.
 * It scores the utterance against scenario examples + hand-written disambiguation
 * cues that mirror `not_this_if` rules, and returns the same RouterDecision shape.
 */
import type { Candidate, Lang, RouterDecision, RoutedScenario, ScenarioId } from "../contract";
import { AS_OF_DATE } from "../contract";
import { PRIORITY_RANK, scenarios, slotByNameOf } from "../catalog";

/* ------------------------------ language ------------------------------ */

const KK_LETTERS = /[әіңғүұқөһ]/i;
const RU_MARKERS = new Set(
  "и ещё еще а не я у вас на мне хочу нужно надо скажите подскажите можно ли что как где когда но помогите пожалуйста здравствуйте добрый день мой моя моё мою мне меня нам наш есть нет да это вот там тут по для от с со из за уже ещё только сейчас вчера завтра сегодня".split(" ")
);

const KK_MARKERS = new Set("керек жазылу болады бар жоқ туралы бойынша деп мен сен сіз біз бізге маған саған оған беру алу кету келу істеу қалай неге сонда осы бұл сол тағы емес ме ба бе па пе".split(" "));

export function detectLanguage(text: string): Lang {
  const words = tokenize(text);
  let kk = 0;
  let ru = 0;
  for (const w of words) {
    if (KK_LETTERS.test(w) || KK_MARKERS.has(w) || /(ке|ға|ге|қа|ды|ді|ты|ті|мын|мін|сыз|сіз|ңыз|ңіз|дым|дім)$/.test(w) && !RU_MARKERS.has(w) && w.length > 4) kk++;
    else if (RU_MARKERS.has(w) || /[ёэ]/.test(w) || /(ть|тся|ый|ий|ой|ать|ить|ует|ает|ите|ете|ого|его|ому|ему|ую|ых|ых)$/.test(w)) ru++;
  }
  if (kk > 0 && ru > 0) return "mixed";
  if (kk > 0) return "kk";
  return "ru";
}

/** Dominant language → reply language. */
export function dominantLanguage(text: string): "ru" | "kk" {
  const words = tokenize(text);
  let kk = 0;
  let ru = 0;
  for (const w of words) {
    if (KK_LETTERS.test(w) || KK_MARKERS.has(w)) kk += 1.5; // Kazakh letters / function words are a strong signal
    else if (/(ке|ға|ге|қа|ды|ді|мын|мін|сыз|сіз|ңыз|ңіз|дым|дім|ылу|ілу)$/.test(w) && w.length > 4) kk += 1;
    else if (RU_MARKERS.has(w) || /[ёэ]/.test(w) || /(ть|тся|ый|ий|ой|ать|ить|ует|ает|ите|ете|ого|его|ому|ему|ую|ых)$/.test(w)) ru += 1;
  }
  return kk > ru ? "kk" : "ru";
}

/* ------------------------------ tokens ------------------------------ */

export function tokenize(text: string): string[] {
  return text
    .toLowerCase()
    .replace(/ё/g, "е")
    .replace(/[^a-zа-яәіңғүұқөһ0-9+\s]/gi, " ")
    .split(/\s+/)
    .filter(Boolean);
}

function stem(w: string): string {
  if (/^\d+$/.test(w)) return w;
  return w.length > 5 ? w.slice(0, 5) : w;
}

const STOP = new Set(
  "и ещё еще а не я у вас на мне хочу нужно нужна нужен надо скажите подскажите можно ли что как где когда но помогите пожалуйста здравствуйте добрый день мой моя мою мне меня нам наш есть нет да это вот там тут по для от с со из за уже только сейчас в во к ко о об же бы то ну ваш ваша вашему вашего сәлеметсіз бе маған менің мен сіздер сіздерде керек болады бола ма ме па пе бар жоқ деп едім еді ғой да де әрі және алдым білгім келеді келді".split(" ")
);

interface ScenarioIndex {
  id: ScenarioId;
  stems: Map<string, number>; // stem → weight
}

let INDEX: ScenarioIndex[] | null = null;
let DF: Map<string, number> | null = null;

function buildIndex(): ScenarioIndex[] {
  if (INDEX) return INDEX;
  const df = new Map<string, number>();
  const idx: ScenarioIndex[] = scenarios.map((s) => {
    const stems = new Map<string, number>();
    const add = (t: string, w: number) => {
      for (const tok of tokenize(t)) {
        if (STOP.has(tok)) continue;
        const st = stem(tok);
        stems.set(st, (stems.get(st) ?? 0) + w);
      }
    };
    for (const ex of s.examples.ru) add(ex, 1);
    for (const ex of s.examples.kk) add(ex, 1);
    for (const st of stems.keys()) df.set(st, (df.get(st) ?? 0) + 1);
    return { id: s.scenario_id, stems };
  });
  INDEX = idx;
  DF = df;
  return idx;
}

/* ------------------------------ cue rules ------------------------------ */
/** Hand-written cues mirroring `not_this_if`. [regex, scenario, weight]. */
const CUES: Array<[RegExp, ScenarioId, number]> = [
  // accident cluster
  [/(только что|прямо сейчас|стою на дороге|стою после|қазір ғана|жолдың ортасында|соқтығысып қалдық|врезались прямо)/i, "SC11", 3],
  [/(виновник|виноват[^ы]|кінәлі|въехал[аи]? в меня|ударил[иа]|пострадавш|зардап|соғып кетті.*полис|артымнан соғып)/i, "SC12", 2.5],
  [/(каско|касқо)/i, "SC13", 1.2],
  [/(каско|касқо).*(поцарап|разби|стук|соғып|сызып|ұрла|угнал|упал|құлад|бұршақ|фар|двор|аулада)|(поцарап|разби|стукн|соғып|сызып|ұрла|угнал|упал|құлад|бұршақ|фар|двор|аулада).*(каско|касқо)/i, "SC13", 3],
  [/(затопил|протекл|потолок|пожар|өрт|су басты|сгорел)/i, "SC14", 2.5],
  [/(за границ|шетелде|эмират|отравил|заболел за|сломал.*(за границ|шетел)|аяғымды сындырып|сейчас в (турции|эмиратах|египте|таиланде|грузии))/i, "SC15", 4],
  [/(сломал[аи]? (ногу|руку)|жарақат алдым|получил[аи]? травм|травм.*(полис|выплат|заявл)|футбол|(у него|у меня|у неё).*полис.*(несчастн|травм))/i, "SC16", 3.5],
  // claims cluster
  [/(статус|когда переведут|когда будет выплат|что с моим заявлен|рассмотрели|қашан түседі|қандай күйде|не болды)/i, "SC17", 2.5],
  [/(какие (бумаги|документы)|документы (нужны|собирать)|құжаттар|бумаги|донести|ещё нужно (принести|донести|предоставить)|что-то ещё нужно)/i, "SC18", 3.5],
  [/(не согласен|не согласна|оспорить|претензи|слишком мал|меньше, чем|отказали в выплат|келіспеймін|бас тартты|негізсіз|тым аз)/i, "SC19", 3],
  [/(осмотр|показать машину|эксперт|бағалауға|қай күні әкел)/i, "SC20", 2.5],
  // health cluster
  [/(записат|запишите|жазыл|жазып|к врачу|к лору|к терапевт|к кардиолог|гинеколог|стоматолог)/i, "SC21", 2.5],
  [/(покрыва|покрыто|бесплатно по страховк|өтей ме|талдаулар|узи|анализ|лекарств)/i, "SC22", 2.5],
  [/(клиник|емхана)/i, "SC23", 2.5],
  [/(карт[ау] дмс|электронн.*карт|карта медстрах|сақтандыру карт|картасы)/i, "SC24", 3],
  // policy / documents cluster
  [/(действует ли|активн|до какого|не закончилась|мерзімі|жарамды|белсенді|проверьте.*полис|полис.*проверьте|тексеріп)/i, "SC25", 2.5],
  [/(не пришёл|не пришел|не пришло|не приходит полис|отправьте ещё раз|қайта жібер|келмеді|потерял полис|пришлите повторно|рәсімделді, бірақ|оформлен, но)/i, "SC26", 3.5],
  [/(продлить|продлени|ұзарт|истекает|заканчивается)/i, "SC27", 2.5],
  [/(расторг|закрыть договор|вернуть деньги|бұз|бас тартқым|ақшаны қайтар|отказаться от страховк)/i, "SC28", 3],
  [/(новую почту|поменять почту|новый номер|сменил номер телефона|переехал|көштім|мекенжай|поштам өзгерді|нөмірім өзгерді|электронн.*почт|изменить.*(данные|адрес|телефон|почту))/i, "SC29", 2.5],
  [/(деньги (списал|сняли|ушли)|списали|с карты сняли|ақша (шеш|алын)|оплата не прошла|полиса (нет|нигде)|два раза списали|платёж прошёл, а полиса|полис жоқ)/i, "SC30", 3.5],
  [/(рассрочк|бөліп төле|способ.*оплат|картой другого банка|как (можно )?оплатить|қалай төле|оплатить можно)/i, "SC31", 2.5],
  [/(класс|бонус|малус|почему (подорож|выросл|цена)|неге өсті)/i, "SC32", 3],
  [/(офис|адрес|кеңсе|где находится|режим работы|во сколько открыв|сағат нешеде)/i, "SC33", 2.5],
  [/(приложени|личный кабинет|не могу (зайти|войти)|код (не приходит|неверный)|қосымша|кіре алмай|жеке кабинет)/i, "SC34", 2.5],
  [/(жалоб|пожаловаться|шағым|груб|нагруб|ужасн|никто не (звонит|перезвон)|обещали перезвонить|хабарласпады)/i, "SC35", 3.5],
  [/(перезвон|наберите|позвоните (мне )?(позже|вечером|завтра)|хабарласыңызшы|на совещании)/i, "SC36", 3],
  [/(соедини|переключи|дайте (менеджера|оператора|специалиста)|с оператором|поговорить с (человеком|оператором|живым)|живым человеком|операторды қос|операторға қос|тірі адам|маманға қос)/i, "SC37", 3.5],
  [/(мошенн|подозрительн|представился|код из смс|код из sms|перевести деньги|сілтеме|алаяқ|сіздерден деп|якобы из страхов|заблокирован)/i, "SC38", 3.5],
  [/(справк|анықтама|копи[юя]|для визы|посольств|на английском|дубликат)/i, "SC39", 3],
  [/(что (такое|значит)|лимит|франшиз|исключени|не покрыва|в каких случаях|деген не|қандай жағдайлар)/i, "SC40", 2.5],
  // sales cluster
  [/(сколько стоит|почём|цена|стоимост|посчита|рассчита|бағасы|қанша|есепте)/i, "SC01", 1],
  [/(огпо|обязательн|міндетті|кқи)/i, "SC01", 1.5],
  [/(купить|оформить|оформим|рәсімде|сатып ал)/i, "SC02", 1.2],
  [/(огпо|обязательн|міндетті).*(купить|оформить|рәсімде|сатып)|(купить|оформить|рәсімде).*(огпо|обязательн|міндетті)/i, "SC02", 2.5],
  [/(каско|касқо).*(стоит|интересует|посчита|есепте|цена|бағасы|консульт|франшиз)|(посчита|есепте|интересует).*(каско|касқо)/i, "SC03", 3],
  [/(вписать|добавить.*(водител|сына|дочь|жену|мужа)|получила права|жүргізуші ретінде қосу|қосу керек)/i, "SC04", 3],
  [/(сменил номер[а]? на машине|номера на машине|новую машину|ауыстырдым|госномер|другую машину|поменял машину)/i, "SC05", 3],
  [/(поездк|путешеств|летим|шенген|виза|за границу|турци|грузи|түркия|шетелге шығ|сапар|сақтандыру керек.*бару|бару)/i, "SC06", 2.5],
  [/(квартир|дом |жиль|имуществ|үй|пәтер)/i, "SC07", 1.5],
  [/(застраховать.*(квартир|дом|жиль)|(квартир|дом).*(застраховать|страховк)|үйімді сақтандыру|пәтерді сақтандыру)/i, "SC07", 2.5],
  [/(от несчастн|от травм|жазатайым|горных лыж|спорт|подбираю страховку)/i, "SC08", 2.5],
  [/(медстрах|дмс|мед.*страховк|денсаулық)/i, "SC09", 1.2],
  [/(сотрудников|застраховать сотрудник|компани[яию]|предприят|юр.*лиц|корпоратив|кәсіпорн|қызметкерлер)/i, "SC10", 3],
];

const OUT_OF_SCOPE = /(кредит|займ|ипотек|депозит|погод|ауа райы|жұмысқа|вакансия|работу у вас|өмірді сақтандыру|страхование жизни|пенсион|аннуитет|курс доллара|такси)/i;
const UNCLEAR = /(^|[,.!]\s*)(алло|я по поводу|у меня (вопрос|проблема)|бір нәрсе сұрайын|ну там|с машиной вопрос|по поводу страховки|сұрағым бар)/i;
const GOODBYE = /^(спасибо|рахмет|рақмет|всё|все|до свидания|сау болыңыз|нет, спасибо|жоқ, рақмет|отлично, спасибо|хорошо, спасибо|пока)(?![a-zа-яәіңғүұқөһ])/i;
export const YES = /^(да|иә|ия|верно|давайте|оформляйте|подтверждаю|растаймын|жазыңыз|тіркеңіз|ок|окей|хорошо|конечно|ага|дұрыс|болады|иә, растаймын|да, верно|да, давайте)/i;
export const NO = /^(нет|жоқ|не надо|отмен|потом|давайте потом|кейін|не сейчас|пока не)/i;

/* ------------------------------ multi-intent split ------------------------------ */

const SPLIT = /\s*(?:,\s*)?(?:и ещё|и еще|а ещё|а еще|и заодно|заодно|плюс ещё|әрі|және тағы|және|тағы|а также|и также|,\s+и\s+|\s+и\s+(?=(?:ещё|еще|заодно|также|хочу|скажите|подскажите|нужно|надо|проверьте|запишите|посчитайте|где|когда|какие|можно|себя|обязательн|сказать|узнать|поменять|добавить))|,\s+(?=(?:қалай|как |где |қайда|когда |қашан|сколько|қанша|бола ма)))\s*/i;

export function splitParts(text: string): string[] {
  const parts = text
    .split(SPLIT)
    .map((p) => p.trim())
    .filter((p) => tokenize(p).filter((t) => !STOP.has(t)).length >= 2);
  return parts.length ? parts : [text];
}

/* ------------------------------ scoring ------------------------------ */

function scorePart(text: string): Map<ScenarioId, number> {
  const idx = buildIndex();
  const df = DF!;
  const N = idx.length;
  const toks = tokenize(text).filter((t) => !STOP.has(t)).map(stem);
  const scores = new Map<ScenarioId, number>();
  for (const s of idx) {
    let sc = 0;
    for (const t of toks) {
      const w = s.stems.get(t);
      if (w) sc += 0.45 * Math.log(1 + N / (df.get(t) ?? 1)) * Math.min(w, 2);
    }
    scores.set(s.id, sc);
  }
  for (const [re, id, w] of CUES) {
    if (re.test(text)) scores.set(id, (scores.get(id) ?? 0) + w);
  }
  return scores;
}

function toCandidates(scores: Map<ScenarioId, number>): Candidate[] {
  const arr = [...scores.entries()].filter(([, v]) => v > 0).sort((a, b) => b[1] - a[1]);
  if (!arr.length) return [];
  const top = arr[0][1];
  const second = arr[1]?.[1] ?? 0;
  // strength of the best match (cues ≈ 2.5–4) + margin over the runner-up → calibrated-looking probability
  const margin = 1 - second / top;
  const topConf = Math.min(0.97, 0.3 + 0.45 * Math.min(1, top / 3) + 0.25 * margin);
  return arr.slice(0, 5).map(([id, v], i) => ({
    scenario_id: id,
    confidence: round2(i === 0 ? topConf : Math.max(0.05, topConf * (v / top) * 0.9)),
  }));
}

function round2(x: number) {
  return Math.round(x * 100) / 100;
}

/* ------------------------------ spoken numbers ------------------------------ */

const NUM_WORDS: Record<string, number> = {
  ноль: 0, нөл: 0, один: 1, одна: 1, бір: 1, два: 2, две: 2, екі: 2, три: 3, үш: 3, четыре: 4, төрт: 4, пять: 5, бес: 5, шесть: 6, алты: 6, семь: 7, жеті: 7, восемь: 8, сегіз: 8, девять: 9, тоғыз: 9,
  десять: 10, он: 10, одиннадцать: 11, двенадцать: 12, тринадцать: 13, четырнадцать: 14, пятнадцать: 15, шестнадцать: 16, семнадцать: 17, восемнадцать: 18, девятнадцать: 19,
  двадцать: 20, жиырма: 20, тридцать: 30, отыз: 30, сорок: 40, қырық: 40, пятьдесят: 50, елу: 50, шестьдесят: 60, алпыс: 60, семьдесят: 70, жетпіс: 70, восемьдесят: 80, сексен: 80, девяносто: 90, тоқсан: 90,
  сто: 100, жүз: 100, двести: 200, триста: 300, четыреста: 400, пятьсот: 500, шестьсот: 600, семьсот: 700, восемьсот: 800, девятьсот: 900,
};

/**
 * "плюс семь семьсот семь, сто двадцать три, сорок пять, шестьдесят семь" → "+7 707 123 45 67"
 * "плюс жеті, жеті жүз бір, нөл нөл нөл, нөл нөл, он" → "+7 701 000 00 10"
 * Only runs of ≥ 3 number words are converted (so Kazakh "он" = 10 never hits the Russian pronoun).
 */
export function wordsToDigits(text: string): string {
  const raw = text.toLowerCase().replace(/ё/g, "е").split(/(\s+|,|[.!?:;])/);
  const out: string[] = [];
  let i = 0;
  while (i < raw.length) {
    // collect a run of number words / commas / spaces
    let j = i;
    let count = 0;
    while (j < raw.length && (raw[j].trim() === "" || /^[,.!?:;]$/.test(raw[j]) || NUM_WORDS[raw[j]] !== undefined)) {
      if (NUM_WORDS[raw[j]] !== undefined) count++;
      j++;
    }
    if (count >= 3) {
      let cur: number | null = null;
      let digits = "";
      const flush = () => { if (cur !== null) { digits += String(cur); cur = null; } };
      for (let k = i; k < j; k++) {
        const w = raw[k];
        if (/^[,.!?:;]$/.test(w)) { flush(); continue; }
        const n = NUM_WORDS[w];
        if (n === undefined) continue;
        if (n === 0) { flush(); digits += "0"; continue; }
        if (n >= 100) {
          if (w === "жүз" && cur !== null && cur >= 1 && cur <= 9) { cur = cur * 100; continue; }
          flush(); cur = n; continue;
        }
        if (n >= 10) {
          if (cur !== null && cur >= 100 && cur % 100 === 0) { cur += n; continue; }
          flush(); cur = n; continue;
        }
        if (cur !== null && ((cur >= 20 && cur % 10 === 0) || (cur >= 100 && cur % 100 === 0))) { cur += n; continue; }
        flush(); cur = n;
      }
      flush();
      out.push(" " + digits + " ");
      i = j;
    } else {
      out.push(raw.slice(i, Math.max(j, i + 1)).join(""));
      i = Math.max(j, i + 1);
    }
  }
  return out.join("");
}

/* ------------------------------ slots ------------------------------ */

const CITY_ALIASES: Array<[RegExp, string]> = [
  [/(алмат|almaty)/i, "Almaty"],
  [/(астан|astana|нур-султан)/i, "Astana"],
  [/(шымкент|shymkent)/i, "Shymkent"],
  [/(караганд|қарағанд|karaganda)/i, "Karaganda"],
  [/(актобе|ақтөбе|aktobe)/i, "Aktobe"],
  [/(атырау|atyrau)/i, "Atyrau"],
  [/(павлодар|pavlodar)/i, "Pavlodar"],
  [/(усть-каменогорск|өскемен|oskemen)/i, "Oskemen"],
];

const SPECIALTY: Array<[RegExp, string]> = [
  [/(терапевт)/i, "therapist"],
  [/(лор|отоларинголог)/i, "ENT"],
  [/(кардиолог)/i, "cardiologist"],
  [/(гинеколог)/i, "gynecologist"],
  [/(стоматолог|зубн|тіс)/i, "dentist"],
  [/(педиатр)/i, "pediatrician"],
];

const COUNTRY: Array<[RegExp, string]> = [
  [/(турци|түркия)/i, "Turkey"],
  [/(грузи)/i, "Georgia"],
  [/(эмират|оаэ|дубай)/i, "UAE"],
  [/(египет|мысыр)/i, "Egypt"],
  [/(таиланд|тайланд)/i, "Thailand"],
  [/(шенген|герман|франц|итали|испани)/i, "Schengen"],
  [/(сша|америк)/i, "USA"],
];

function shiftDate(days: number): string {
  const d = new Date(AS_OF_DATE + "T00:00:00Z");
  d.setUTCDate(d.getUTCDate() + days);
  return d.toISOString().slice(0, 10);
}

export function extractSlots(text: string): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  const spoken = wordsToDigits(text);
  const digits = spoken.replace(/плюс|\+/gi, "+").replace(/[^\d+]/g, "");
  const phone = digits.match(/(?:\+?7|8)(\d{10})/);
  if (phone) out.phone = "+7" + phone[1];
  const iin = spoken.replace(/\s/g, "").match(/(?<!\d)(\d{12})(?!\d)/);
  if (iin && !phone) out.iin = iin[1];
  const claim = text.match(/CL[-\s]?(\d{6})/i);
  if (claim) out.claim_number = "CL-" + claim[1];
  const policy = text.match(/SQ[-\s]?(OGPO|CASCO|TRVL|PROP|NS|DMS)[-\s]?(\d{6})/i);
  if (policy) out.policy_number = `SQ-${policy[1].toUpperCase()}-${policy[2]}`;
  const plate = text.replace(/\s/g, "").match(/(\d{3}[A-Z]{2,3}\d{2})/i);
  if (plate) out.vehicle_plate = plate[1].toUpperCase();
  for (const [re, city] of CITY_ALIASES) if (re.test(text)) { out.city = city; break; }
  for (const [re, sp] of SPECIALTY) if (re.test(text)) { out.doctor_specialty = sp; break; }
  for (const [re, c] of COUNTRY) if (re.test(text)) { out.trip_country = c; break; }
  if (/(завтра|ертең)/i.test(text)) out.preferred_date = shiftDate(1);
  if (/(вчера|кеше)/i.test(text)) out.incident_date = shiftDate(-1);
  if (/(сегодня|бүгін)/i.test(text)) out.incident_date = AS_OF_DATE;
  if (/(легков)/i.test(text)) out.vehicle_type = "car";
  if (/(грузов|жүк)/i.test(text)) out.vehicle_type = "truck";
  if (/(мотоцикл)/i.test(text)) out.vehicle_type = "motorcycle";
  if (/(алмат)/i.test(text)) out.region = "almaty";
  else if (/(астан)/i.test(text)) out.region = "astana";
  const year = text.match(/(20[0-2]\d)\s*(года|год|жыл)?/);
  if (year) out.car_year = Number(year[1]);
  if (/(двадцатого года)/i.test(text)) out.car_year = 2020;
  const mail = text.match(/[\w.+-]+@[\w-]+\.[\w.]+/);
  if (mail) out.email = mail[0];
  const email = /(почт|email|мейл|пошта)/i.test(text);
  if (email && /(помен|измен|нов|өзгер)/i.test(text)) out.contact_field = "email";
  else if (/(телефон|нөмір)/i.test(text) && /(помен|измен|нов|өзгер)/i.test(text)) out.contact_field = "phone";
  else if (/(адрес|мекенжай)/i.test(text) && /(помен|измен|нов|переехал|көштім|өзгер)/i.test(text)) out.contact_field = "address";
  const sum = text.match(/(\d{1,3})\s*(млн|миллион)/i);
  if (sum) out.sum_insured = Number(sum[1]) * 1_000_000;
  return out;
}

/** Validate a free-text answer against a slot definition (for continuation). */
export function parseSlotAnswer(slotName: string, text: string): unknown | null {
  const def = slotByNameOf(slotName);
  const ex = extractSlots(text);
  if (slotName in ex) return ex[slotName];
  if (!def) return text.trim();
  if (def.type === "list") {
    const all = [...text.replace(/\s(?=\d)/g, "").matchAll(/(?<!\d)(\d{12})(?!\d)/g)].map((m) => m[1]);
    if (all.length) return all;
    const spokenAll = [...wordsToDigits(text).replace(/\s(?=\d)/g, "").matchAll(/(?<!\d)(\d{12})(?!\d)/g)].map((m) => m[1]);
    return spokenAll.length ? spokenAll : null;
  }
  if (def.type === "boolean") {
    if (/(все целы|целы|никто|жоқ|нет|аман)/i.test(text)) return false;
    if (/(пострадав|есть|да|иә|ранен|зардап)/i.test(text)) return true;
    return null;
  }
  if (def.type === "integer") {
    const n = text.replace(/\s/g, "").match(/\d+/);
    return n ? Number(n[0]) : null;
  }
  if (def.type === "enum" && def.values) {
    const low = text.toLowerCase();
    const hit = def.values.find((v) => low.includes(String(v).toLowerCase()));
    if (hit !== undefined) return hit;
    if (slotName === "product_type") {
      if (/(огпо|обязат|міндетті)/i.test(text)) return "ogpo";
      if (/(каско)/i.test(text)) return "casco";
      if (/(поезд|путеш|сапар|travel)/i.test(text)) return "travel";
      if (/(квартир|дом|үй|пәтер|имущ)/i.test(text)) return "property";
      if (/(несчаст|травм|жазатайым)/i.test(text)) return "accident";
      if (/(дмс|мед)/i.test(text)) return "dms";
    }
    if (slotName === "property_type") {
      if (/(квартир|пәтер)/i.test(text)) return "apartment";
      if (/(дом|үй)/i.test(text)) return "house";
    }
    if (slotName === "sum_insured") {
      const n = text.replace(/\s/g, "").match(/\d+/);
      if (n) { const v = Number(n[0]); return v < 1000 ? v * 1_000_000 : v; }
    }
    if (slotName === "franchise") {
      if (/(без|нет|жоқ)/i.test(text)) return 0;
    }
    return null;
  }
  if (def.type === "date") {
    if (/(завтра|ертең)/i.test(text)) return shiftDate(1);
    if (/(послезавтра|бүрсігүні)/i.test(text)) return shiftDate(2);
    if (/(вчера|кеше)/i.test(text)) return shiftDate(-1);
    if (/(сегодня|бүгін)/i.test(text)) return AS_OF_DATE;
    const m = text.match(/(\d{4})-(\d{2})-(\d{2})/);
    if (m) return m[0];
    const d = text.match(/(\d{1,2})\s*(октябр|қазан|сентябр|қыркүйек|ноябр|қараша)/i);
    if (d) {
      const month = /(октябр|қазан)/i.test(d[2]) ? "10" : /(сентябр|қыркүйек)/i.test(d[2]) ? "09" : "11";
      return `2026-${month}-${d[1].padStart(2, "0")}`;
    }
    return null;
  }
  if (def.pattern) {
    const re = new RegExp(def.pattern);
    const compact = text.replace(/\s/g, "");
    return re.test(compact) ? compact : null;
  }
  const t = text.trim();
  return t.length ? t : null;
}

/* ------------------------------ decision ------------------------------ */

export interface MockRouteResult {
  decision: RouterDecision;
  snapshots: Candidate[][]; // for the "live confidence" animation
  parts: string[];
  urgent: boolean;
  kind: "route" | "yes" | "no" | "goodbye";
}

export function mockRoute(text: string, ctx: { awaiting: boolean }): MockRouteResult {
  const language = detectLanguage(text);
  const trimmed = text.trim();

  if (ctx.awaiting && NO.test(trimmed) && tokenize(trimmed).length <= 6) {
    return { kind: "no", parts: [trimmed], urgent: false, snapshots: [], decision: emptyDecision(language, true, "confirmation: no") };
  }
  if (ctx.awaiting && YES.test(trimmed) && tokenize(trimmed).length <= 6) {
    return { kind: "yes", parts: [trimmed], urgent: false, snapshots: [], decision: emptyDecision(language, true, "confirmation: yes") };
  }
  if (ctx.awaiting && NO.test(trimmed) && tokenize(trimmed).length <= 6) {
    return { kind: "no", parts: [trimmed], urgent: false, snapshots: [], decision: emptyDecision(language, true, "confirmation: no") };
  }
  if (GOODBYE.test(trimmed) && tokenize(trimmed).length <= 4) {
    return {
      kind: "goodbye", parts: [trimmed], urgent: false, snapshots: [],
      decision: { ...emptyDecision(language, false, "client ends the conversation"), scenarios: [{ scenario_id: "SYS_GOODBYE", confidence: 0.95, reason: "farewell" }] },
    };
  }

  const parts = splitParts(trimmed);
  const perPart = parts.map((p) => toCandidates(scorePart(p)));
  const whole = toCandidates(scorePart(trimmed));

  const chosen: RoutedScenario[] = [];
  const seen = new Set<ScenarioId>();
  for (let i = 0; i < parts.length; i++) {
    const c = perPart[i][0];
    if (!c) continue;
    if (parts.length > 1 && c.confidence < 0.35) continue;
    if (seen.has(c.scenario_id)) continue;
    seen.add(c.scenario_id);
    chosen.push({ scenario_id: c.scenario_id, confidence: c.confidence, reason: reasonFor(c.scenario_id, parts[i]) });
  }
  if (!chosen.length && whole[0]) chosen.push({ scenario_id: whole[0].scenario_id, confidence: whole[0].confidence, reason: reasonFor(whole[0].scenario_id, trimmed) });

  // tail intents that hide inside another request: "…продлить в рассрочку?" → + SC31
  if (/(рассрочк|бөліп төле)/i.test(trimmed) && !seen.has("SC31") && chosen.length && chosen[0].scenario_id !== "SC31") {
    seen.add("SC31");
    chosen.push({ scenario_id: "SC31", confidence: 0.8, reason: "Способы оплаты и рассрочка — в конце реплики есть «рассрочка»" });
  }

  // urgent first, then mention order
  chosen.sort((a, b) => {
    const pa = PRIORITY_RANK[scenarios.find((s) => s.scenario_id === a.scenario_id)?.priority ?? "normal"];
    const pb = PRIORITY_RANK[scenarios.find((s) => s.scenario_id === b.scenario_id)?.priority ?? "normal"];
    return (pa === 0 ? 0 : 1) - (pb === 0 ? 0 : 1);
  });

  const alternatives = whole.filter((c) => !seen.has(c.scenario_id)).slice(0, 3);
  const top = chosen[0];
  const scenariosOut: RoutedScenario[] = [...chosen];
  let reason = top ? top.reason ?? "" : "";

  const contentTokens = tokenize(trimmed).filter((t) => !STOP.has(t));
  if (OUT_OF_SCOPE.test(trimmed) && (!top || top.confidence < 0.85)) {
    scenariosOut.splice(0, scenariosOut.length, { scenario_id: "SYS_OUT_OF_SCOPE", confidence: 0.9, reason: "not a Saqta service" });
    reason = "Request is outside Saqta's products (loans, life insurance, weather, jobs)";
  } else if (!top || top.confidence < 0.45 || (contentTokens.length <= 2 && top.confidence < 0.7) || (UNCLEAR.test(trimmed) && top.confidence < 0.8)) {
    scenariosOut.splice(0, scenariosOut.length, { scenario_id: "SYS_UNCLEAR", confidence: top ? round2(1 - top.confidence) : 0.8, reason: "too vague" });
    reason = "Too vague to pick a scenario; ask one clarifying question with the two closest options";
  }

  const urgent = scenariosOut.some((s) => scenarios.find((x) => x.scenario_id === s.scenario_id)?.priority === "urgent");

  // snapshots: simulate the router "converging" (for the admin live view)
  const finalCands: Candidate[] = whole.length ? whole : scenariosOut.map((s) => ({ scenario_id: s.scenario_id, confidence: s.confidence }));
  const snapshots: Candidate[][] = [0.35, 0.6, 0.85, 1].map((k) =>
    finalCands.map((c, i) => ({ scenario_id: c.scenario_id, confidence: round2(c.confidence * k + (i === 0 ? 0 : (1 - k) * 0.15)) }))
  );

  return {
    kind: "route",
    parts,
    urgent,
    snapshots,
    decision: {
      scenarios: scenariosOut,
      alternatives,
      language,
      slots: extractSlots(trimmed),
      is_continuation: false,
      reason,
      model: "mock-lexical",
      tier: "fast",
    },
  };
}

function emptyDecision(language: Lang, cont: boolean, reason: string): RouterDecision {
  return { scenarios: [], alternatives: [], language, slots: {}, is_continuation: cont, reason, model: "mock-lexical", tier: "fast" };
}

function reasonFor(id: ScenarioId, part: string): string {
  const s = scenarios.find((x) => x.scenario_id === id);
  const cues = CUES.filter(([re, sid]) => sid === id && re.test(part)).map(([re]) => re.source.split("|")[0].replace(/[()^$]/g, ""));
  const cue = cues[0] ? `в реплике есть признак «${cues[0]}»` : "похоже на примеры этого сценария из каталога";
  return `${s ? s.name : id} — ${cue}`;
}
