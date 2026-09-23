import Link from "next/link";
import { PlusMark } from "./header";

const COLS = [
  { h: "Продукт", items: [["Симулятор звонка", "/call"], ["Консоль супервизора", "/admin"], ["Лендинг", "/"]] },
  { h: "Кейс", items: [["Halyk Bank · Voice Router", "https://edu.astanahub.com/hackathons/df4743f5-c492-415c-b45a-1f13adb78e06?tab=tracks"], ["Стартовый кит и evaluate.py", "https://github.com/BAITC-Hacks/hack-9dd7b5f3-plus/tree/main/data"], ["Контракт API", "https://github.com/BAITC-Hacks/hack-9dd7b5f3-plus/blob/main/docs/API_CONTRACT.md"]] },
  { h: "Команда Plus", items: [["GitHub", "https://github.com/BAITC-Hacks/hack-9dd7b5f3-plus"], ["HackAlem AI 2026", "https://hackalem.ai/"]] },
] as const;

export function Footer() {
  return (
    <footer className="relative z-10 border-t border-border bg-paper">
      <div className="mx-auto grid w-full max-w-[1200px] gap-10 px-5 py-14 sm:px-8 md:grid-cols-[minmax(0,5fr)_repeat(3,minmax(0,2fr))]">
        <div>
          <div className="flex items-center gap-2 text-ink"><span className="grid size-6 place-items-center rounded-md bg-brand text-white"><PlusMark className="size-3" /></span><span className="font-medium">Bagyt</span></div>
          <p className="mt-3 max-w-[32ch] text-sm leading-relaxed text-body">Голосовой робот контакт-центра с LLM-слоем выбора сценария. Русский и казахский.</p>
        </div>
        {COLS.map((c) => (
          <div key={c.h}>
            <div className="text-xs uppercase tracking-[0.12em] text-body/70">{c.h}</div>
            <ul className="mt-3 space-y-2 text-sm">
              {c.items.map(([label, href]) => (
                <li key={label}>{href.startsWith("/") ? <Link href={href as "/"} className="text-ink hover:text-brand">{label}</Link> : <a href={href} target="_blank" rel="noreferrer" className="text-ink hover:text-brand">{label}</a>}</li>
              ))}
            </ul>
          </div>
        ))}
      </div>
      <div className="border-t border-border">
        <div className="mx-auto flex w-full max-w-[1200px] items-center justify-between px-5 py-4 text-xs text-body sm:px-8">
          <span className="flex items-center gap-2"><span className="size-1.5 rounded-full bg-ok" /> Демо работает в браузере без ключей</span>
          <span>Team Plus · HackAlem AI · 2026</span>
        </div>
      </div>
    </footer>
  );
}
