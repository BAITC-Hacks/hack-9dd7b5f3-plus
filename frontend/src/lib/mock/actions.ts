/**
 * MOCK backend actions over data/mock_backend.json + data/knowledge_base.json.
 * Same names/inputs/outputs/errors as data/actions.json. Deterministic.
 */
import type { ActionCall } from "../contract";
import { backend, kb, type Claim, type Client, type Policy } from "../catalog";

type R = Record<string, unknown>;
const err = (code: string, message: string) => ({ error: { code, message } });

let seq = 900;
const nextId = (prefix: string) => `${prefix}${(seq++).toString().padStart(6, "0").slice(-6)}`;

function findClient(input: R): R | { error: { code: string; message: string } } {
  const phone = input.phone as string | undefined;
  const iin = input.iin as string | undefined;
  const c = backend.clients.find((x) => (phone && x.phone === phone) || (iin && x.iin === iin));
  if (!c) return err("not_found", `Client with ${phone ? "phone " + phone : "IIN " + iin} not found`);
  return { client_id: c.client_id, full_name: c.full_name, preferred_language: c.preferred_language };
}

function getPolicies(input: R) {
  const list = backend.policies.filter((p) => p.client_id === input.client_id);
  if (!list.length) return err("not_found", "No policies for client");
  return { policies: list.map((p) => ({ policy_number: p.policy_number, product: p.product, status: statusOf(p), end_date: p.end_date })) };
}

function getPolicy(input: R) {
  const p = backend.policies.find((x) => x.policy_number === input.policy_number || (input.vehicle_plate && x.details.vehicle_plate === input.vehicle_plate));
  if (!p) return err("not_found", "Policy not found");
  return { policy_number: p.policy_number, product: p.product, status: statusOf(p), end_date: p.end_date, premium: p.premium, details: p.details };
}

function statusOf(p: Policy): string {
  return p.end_date >= "2026-10-01" ? "active" : "expired";
}

function getClaim(input: R) {
  const c: Claim | undefined = backend.claims.find((x) => x.claim_number === input.claim_number) ?? backend.claims.find((x) => x.client_id === input.client_id);
  if (!c) return err("not_found", "Claim not found");
  return { claim_number: c.claim_number, status: c.status, next_step: c.next_step, approved_amount: c.approved_amount, missing_documents: c.missing_documents, decision_due: c.decision_due };
}

function checkPayment(input: R) {
  const p = backend.payments.find((x) => x.client_id === input.client_id);
  if (!p) return err("not_found", "Payment not found");
  return { payment_status: p.status, amount: p.amount, date: p.date, note: p.note };
}

function getBmClass(input: R) {
  const c = backend.clients.find((x) => x.iin === input.iin);
  return { bm_class: c?.bm_class ?? backend.clients.length ? (c?.bm_class ?? "3") : "3" };
}

function calcOgpo(input: R) {
  const region = (input.region as string) ?? "other";
  const base = kb.products.ogpo.pricing.base_by_region_kzt[region] ?? kb.products.ogpo.pricing.base_by_region_kzt.other;
  const vt = kb.products.ogpo.pricing.vehicle_type_coef[(input.vehicle_type as string) ?? "car"] ?? 1;
  const iins = (input.drivers_iin as string[]) ?? [];
  const classes = iins.map((i) => backend.clients.find((c) => c.iin === i)?.bm_class ?? "3");
  const worst = classes.length ? classes.reduce((a, b) => (Number(kb.products.ogpo.pricing.bm_coef[a]) >= Number(kb.products.ogpo.pricing.bm_coef[b]) ? a : b)) : "3";
  const bm = kb.products.ogpo.pricing.bm_coef[worst] ?? 1;
  return { price: Math.round(base * vt * bm), bm_class: worst };
}

function calcCasco(input: R) {
  const value = Number(input.car_value ?? 0);
  const year = Number(input.car_year ?? 2020);
  const age = 2026 - year;
  if (age > 10) return err("not_eligible", "Car older than 10 years is not eligible for CASCO Standard");
  const rate = age <= 3 ? 0.04 : age <= 7 ? 0.05 : 0.065;
  const fr = kb.products.casco.pricing.franchise_coef[String(input.franchise ?? 0)] ?? 1;
  return { price: Math.round(value * rate * fr) };
}

function calcTravel(input: R) {
  const country = String(input.trip_country ?? "");
  const zone = /(Georgia|CIS|Uzbekistan|Kyrgyzstan)/i.test(country) ? "A" : /(Schengen|UK|Germany|France|Italy|Spain)/i.test(country) ? "B" : /(USA|Canada)/i.test(country) ? "D" : "C";
  const z = kb.products.travel.zones[zone];
  const start = new Date(String(input.trip_start ?? "2026-10-10"));
  const end = new Date(String(input.trip_end ?? "2026-10-16"));
  const days = Math.max(1, Math.round((end.getTime() - start.getTime()) / 86400000) + 1);
  const travelers = Number(input.travelers_count ?? 1);
  const age = Number(input.traveler_max_age ?? 30);
  if (age > 75) return err("not_eligible", "Travelers over 75 only via operator");
  const coef = age >= 65 ? 2 : 1;
  return { price: z.rate_per_day_kzt * days * travelers * coef, zone, coverage: z.coverage };
}

function calcProperty(input: R) {
  const sum = String(input.sum_insured ?? "5000000");
  const base = kb.products.property.price_per_year_kzt[sum];
  if (!base) return err("invalid_input", "Unsupported sum insured");
  return { price: input.property_type === "house" ? Math.round(base * kb.products.property.house_coef) : base };
}

function calcAccident(input: R) {
  const base = kb.products.accident.price_per_year_kzt[String(input.sum_insured ?? "1000000")];
  if (!base) return err("invalid_input", "Unsupported sum insured");
  return { price: base };
}

function listClinics(input: R) {
  const list = kb.clinics.filter((c: { city: string }) => c.city === input.city);
  if (!list.length) return err("not_found", "No partner clinics in this city");
  return { clinics: list.map((c: { name: string; address: string }) => `${c.name} (${c.address})`).join(", ") };
}

function getOffices(input: R) {
  const o = kb.offices.find((x: { city: string }) => x.city === input.city);
  if (!o) return err("not_found", "No office in this city");
  return { address: o.address, hours: o.hours };
}

function kbLookup(input: R) {
  const topic = String(input.topic ?? "");
  const lang = (input.lang as "ru" | "kk") ?? "ru";
  const t: Record<string, { ru: string; kk: string }> = {
    "payments.methods": {
      ru: "Оплатить можно картой в приложении или на сайте, по ссылке из SMS, переводом для компаний или в кассе офиса. Наличные не принимаем.",
      kk: "Қосымшада немесе сайтта картамен, SMS-тегі сілтеме арқылы, компаниялар үшін аударыммен немесе кеңседегі терминалда төлеуге болады. Қолма-қол ақша қабылданбайды.",
    },
    "payments.installments": {
      ru: "КАСКО можно оплатить в 2 или 4 платежа без переплаты, ДМС — в 2 платежа. ОГПО и страховка для поездок оплачиваются полностью.",
      kk: "КАСКО-ны 2 немесе 4 төлеммен, ДМС-ті 2 төлеммен бөліп төлеуге болады. ОГПО мен сапар сақтандыруы толық төленеді.",
    },
    "claims.road_accident_now": {
      ru: "Если есть пострадавшие — звоните 112. Включите аварийку, выставьте знак, не убирайте машины до оформления, сфотографируйте место и номера, обменяйтесь контактами с другим водителем.",
      kk: "Зардап шеккендер болса — 112-ге қоңырау шалыңыз. Апаттық сигналды қосыңыз, белгі қойыңыз, рәсімделгенге дейін көліктерді қозғамаңыз, орынды және нөмірлерді суретке түсіріңіз.",
    },
    "claims.documents.ogpo_victim": { ru: "удостоверение, права, техпаспорт, документы из полиции о ДТП, банковские реквизиты и фото повреждений", kk: "жеке куәлік, жүргізуші куәлігі, техпаспорт, полицияның ЖКО құжаттары, банк деректемелері және зақым суреттері" },
    "claims.documents.casco": { ru: "удостоверение, права, техпаспорт, документы из полиции (если вызывали) и фото повреждений", kk: "жеке куәлік, жүргізуші куәлігі, техпаспорт, полиция құжаттары (шақырылса) және зақым суреттері" },
    "claims.documents.property": { ru: "удостоверение, номер полиса, акт от управляющей компании или справка пожарной службы, фото повреждений и банковские реквизиты", kk: "жеке куәлік, полис нөмірі, басқарушы компанияның актісі немесе өрт қызметінің анықтамасы, зақым суреттері және банк деректемелері" },
    "claims.documents.accident": { ru: "удостоверение, справка из травмпункта или больницы и банковские реквизиты", kk: "жеке куәлік, травмпункт немесе аурухана анықтамасы және банк деректемелері" },
    "claims.documents.travel": { ru: "номер полиса, медицинские документы из-за границы и чеки по согласованным расходам", kk: "полис нөмірі, шетелдік медициналық құжаттар және келісілген шығындар чектері" },
    "app_help.login": {
      ru: "Вход по номеру телефона и одноразовому коду из SMS. Если код не приходит: проверьте номер, подождите 60 секунд и запросите новый, максимум 5 кодов в час.",
      kk: "Телефон нөмірі және SMS-тегі бір реттік код арқылы кіріңіз. Код келмесе: нөмірді тексеріңіз, 60 секунд күтіп, жаңа код сұраңыз, сағатына ең көбі 5 код.",
    },
    "fraud_policy": {
      ru: "Мы никогда не просим коды из SMS, CVV или PIN и не просим переводить деньги на личные карты. Полис не аннулируют за отказ назвать код.",
      kk: "Біз ешқашан SMS кодын, CVV немесе PIN сұрамаймыз және жеке картаға ақша аударуды сұрамаймыз. Кодты айтудан бас тартқаны үшін полис жойылмайды.",
    },
    "dms.e_card": { ru: "Электронная карта ДМС есть в приложении в разделе «Мои полисы», можно отправить по SMS. В клинике достаточно показать её с экрана.", kk: "ДМС электрондық картасы қосымшадағы «Менің полистерім» бөлімінде, SMS-пен жіберуге болады. Емханада экраннан көрсету жеткілікті." },
    "dms.individual": { ru: "Есть две программы: Базовая за 180 тысяч тенге в год и Комфорт за 320 тысяч. В Комфорт входят специалисты без направления, УЗИ, МРТ по направлению и стоматология.", kk: "Екі бағдарлама бар: Базалық — жылына 180 мың теңге, Комфорт — 320 мың. Комфортқа жолдамасыз мамандар, УДЗ, жолдамамен МРТ және стоматология кіреді." },
    "terms.franchise": { ru: "Франшиза — это часть ущерба, которую вы оплачиваете сами; чем она выше, тем дешевле полис: без франшизы, 50 или 100 тысяч тенге.", kk: "Франшиза — зақымның өзіңіз төлейтін бөлігі; ол жоғары болған сайын полис арзан: франшизасыз, 50 немесе 100 мың теңге." },
    "terms.exclusions": { ru: "По КАСКО не покрываются вождение в нетрезвом виде, водитель не из полиса, умышленный ущерб, износ и работа в такси без уведомления.", kk: "КАСКО бойынша мас күйде жүргізу, полисте жоқ жүргізуші, әдейі келтірілген зақым, тозу және хабарламай таксиде жұмыс істеу өтелмейді." },
    "terms.limit": { ru: "Лимит ответственности — максимальная сумма выплаты по полису. По ОГПО покрывается вред здоровью и имуществу других участников ДТП, своя машина не покрывается.", kk: "Жауапкершілік лимиті — полис бойынша ең жоғары төлем сомасы. ОГПО бойынша басқа қатысушылардың денсаулығы мен мүлкіне келтірілген зиян өтеледі, өз көлігіңіз өтелмейді." },
    "bonus_malus": { ru: "Стартовый класс 3. За каждый год без аварий по вашей вине класс растёт на один, за аварию по вашей вине снижается на два. Чем выше класс, тем дешевле ОГПО.", kk: "Бастапқы класс — 3. Кінәсіз әр жыл үшін класс бірге өседі, кінәлі апат үшін екіге төмендейді. Класс жоғары болған сайын ОГПО арзан." },
    "cancellation": { ru: "Возврат считается по формуле: премия за неиспользованные полные месяцы минус 10% расходов. Деньги приходят на карту в течение 10 рабочих дней.", kk: "Қайтарым пайдаланылмаған толық айлар үшін сыйлықақыдан 10% шығынды алып есептеледі. Ақша 10 жұмыс күні ішінде картаға түседі." },
  };
  const hit = t[topic];
  if (!hit) return err("not_found", `No KB topic ${topic}`);
  return { answer: hit[lang] };
}

const IRREVERSIBLE = new Set(["create_policy", "renew_policy", "update_policy", "cancel_policy", "create_claim", "create_dispute", "book_inspection", "book_appointment", "update_contact"]);

export function isIrreversible(name: string) {
  return IRREVERSIBLE.has(name);
}

/** Execute a mock action. `mode: "preview"` computes results without side effects. */
export function runAction(name: string, input: R): ActionCall {
  const t0 = performance.now();
  let result: R | { error: { code: string; message: string } };
  switch (name) {
    case "find_client": result = findClient(input); break;
    case "get_policies": result = getPolicies(input); break;
    case "get_policy": result = getPolicy(input); break;
    case "get_bm_class": result = getBmClass(input); break;
    case "calc_ogpo_price": result = calcOgpo(input); break;
    case "calc_casco_price": result = calcCasco(input); break;
    case "calc_travel_price": result = calcTravel(input); break;
    case "calc_property_price": result = calcProperty(input); break;
    case "calc_accident_price": result = calcAccident(input); break;
    case "get_claim": result = getClaim(input); break;
    case "check_payment": result = checkPayment(input); break;
    case "list_clinics": result = listClinics(input); break;
    case "get_offices": result = getOffices(input); break;
    case "kb_lookup": result = kbLookup(input); break;
    case "check_coverage": {
      const svc = String(input.service_name ?? "").toLowerCase();
      const covered = !/(мрт|кт|протез|имплант|косметолог|лекарств)/i.test(svc);
      result = { covered, note: covered ? (input.lang === "kk" ? "Комфорт бағдарламасы бойынша дәрігер жолдамасымен өтеледі." : "По программе Комфорт покрывается по направлению врача.") : (input.lang === "kk" ? "Бұл қызмет сіздің бағдарламаңызға кірмейді." : "Эта услуга не входит в вашу программу.") };
      break;
    }
    case "create_policy": result = { policy_number: `SQ-${String(input.product_type ?? "ogpo").toUpperCase()}-${nextId("")}` }; break;
    case "renew_policy": {
      const old = backend.policies.find((x) => x.policy_number === input.policy_number);
      const product = (old?.product ?? String(input.product ?? "ogpo")).toUpperCase().replace("TRAVEL", "TRVL").replace("PROPERTY", "PROP");
      if (old && old.end_date > "2027-06-01") { result = err("not_eligible", "Policy is not close to expiry"); break; }
      result = { policy_number: `SQ-${product}-${nextId("")}`, price: old?.premium ?? input.price ?? 38000 };
      break;
    }
    case "update_policy": result = { extra_premium: 4200 }; break;
    case "cancel_policy": {
      const p = backend.policies.find((x) => x.policy_number === input.policy_number);
      const premium = p?.premium ?? 100000;
      const monthsLeft = p ? Math.max(0, Math.round((new Date(p.end_date).getTime() - new Date("2026-10-01").getTime()) / (30 * 86400000))) : 6;
      result = { refund_amount: Math.round((premium * monthsLeft) / 12 * 0.9) };
      break;
    }
    case "create_claim": result = { claim_number: `CL-${nextId("5")}` }; break;
    case "create_dispute": result = { ticket_id: `T-${nextId("7")}` }; break;
    case "book_inspection": {
      const p = kb.inspection_points.find((x: { city: string }) => x.city === input.city) ?? kb.inspection_points[2];
      result = { slot_datetime: `${input.preferred_date ?? "2026-10-03"} 10:00`, address: p.address };
      break;
    }
    case "book_appointment": {
      const c = kb.clinics.find((x: { city: string; specialties: string[] }) => (!input.city || x.city === input.city) && (!input.doctor_specialty || x.specialties.includes(String(input.doctor_specialty)))) ?? kb.clinics[0];
      result = { clinic_name: c.name, slot_datetime: `${input.preferred_date ?? "2026-10-02"} 09:30` };
      break;
    }
    case "resend_documents": {
      const c: Client | undefined = backend.clients.find((x) => x.client_id === input.client_id);
      result = c ? { sent_to: c.email } : err("not_found", "Client not found");
      break;
    }
    case "request_document": result = { sent_to: String(input.email ?? "") }; break;
    case "update_contact": result = {}; break;
    case "send_sms": result = {}; break;
    case "create_callback": result = {}; break;
    case "create_complaint": result = { ticket_id: `T-${nextId("7")}` }; break;
    case "report_fraud": result = { ticket_id: `F-${nextId("9")}` }; break;
    case "transfer_to_operator": result = { queue: input.queue, summary: input.summary }; break;
    default: result = err("not_found", `Unknown action ${name}`);
  }
  const ms = Math.round(performance.now() - t0) + 8; // pretend the backend took a few ms
  if ("error" in result) {
    const e = (result as { error: { code: string; message: string } }).error;
    return { name, mode: (input.__mode as ActionCall["mode"]) ?? "read", input: strip(input), error: e, ms };
  }
  return { name, mode: (input.__mode as ActionCall["mode"]) ?? "read", input: strip(input), result: result as R, ms };
}

function strip(input: R): R {
  const out: R = {};
  for (const [k, v] of Object.entries(input)) if (!k.startsWith("__")) out[k] = v;
  return out;
}

/** Mask PII when reading back (README rule). */
export function maskEmail(email: string): string {
  const [u, d] = email.split("@");
  if (!d) return email;
  return `${u[0]}***@${d}`;
}

/** Spell numbers as words is the TTS's job; here we just insert thin spaces for readability. */
export function fmtMoney(n: number): string {
  return n.toLocaleString("ru-RU").replace(/ /g, " ");
}
