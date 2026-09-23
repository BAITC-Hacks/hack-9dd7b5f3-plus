/** Product shot: the supervisor console with a real example, rendered as a static composition. */
const CANDS = [
  { id: "SC12", name: "Обращение пострадавшего по ОГПО виновника", v: 0.9, on: true },
  { id: "SC13", name: "Заявление об ущербе по КАСКО", v: 0.31 },
  { id: "SC11", name: "ДТП произошло только что", v: 0.18 },
  { id: "SC17", name: "Статус страхового случая", v: 0.09 },
];
const SPEED = [
  { k: "Распознавание", v: 210, w: 14 },
  { k: "Выбор сценария", v: 390, w: 26 },
  { k: "Ответ", v: 240, w: 16 },
  { k: "Озвучка", v: 120, w: 8 },
];

export function Product() {
  return (
    <section className="relative z-10 mx-auto w-full max-w-[1200px] px-5 py-16 sm:px-8 lg:py-24">
      <div className="mb-10 max-w-3xl">
        <h2 className="text-3xl font-medium tracking-[-0.02em] text-ink sm:text-[40px] sm:leading-[1.1]">Не вердикт, а объяснение. После каждой реплики.</h2>
        <p className="mt-4 text-[17px] leading-relaxed text-body">Какой сценарий выбран, с какой уверенностью, какие были альтернативы и сколько заняла каждая стадия.</p>
      </div>

      <div className="dark overflow-hidden rounded-3xl border border-black/10 bg-[#0d0d0d] text-[#f5f5f5] shadow-[0_40px_80px_-40px_rgba(14,21,18,0.35)]">
        <div className="flex items-center justify-between border-b border-[#242424] px-5 py-3 text-xs text-[#8b8b8b]">
          <span className="flex items-center gap-2"><span className="size-2 rounded-[2px] bg-brand" /> Bagyt · Консоль · реплика 3</span>
          <span>пример</span>
        </div>
        <div className="grid gap-px bg-[#242424] lg:grid-cols-[minmax(0,5fr)_minmax(0,7fr)]">
          <div className="space-y-3 bg-[#0d0d0d] p-5">
            <div className="text-xs text-[#8b8b8b]">Разговор</div>
            <div className="flex justify-end"><div className="max-w-[88%] rounded-2xl bg-[#f5f5f5] px-4 py-2.5 text-[15px] leading-relaxed text-[#0d0d0d]">Кеше аварияға түстім, но я не виноват, виновник у вас застрахован</div></div>
            <div className="flex"><div className="max-w-[88%] rounded-2xl bg-[#171717] px-4 py-2.5 text-[15px] leading-relaxed">Түсіндім, сіз кінәлі емессіз. Кінәлінің ОГПО полисі бойынша өтініш рәсімдейік. Кінәлі көліктің нөмірін айтыңызшы.</div></div>
            <div className="pt-2 text-xs text-[#8b8b8b]">Язык ответа: казахский · клиент ещё не определён</div>
          </div>
          <div className="space-y-4 bg-[#0d0d0d] p-5">
            <div className="rounded-xl border border-[#242424] bg-[#171717] p-4">
              <div className="flex items-start justify-between gap-4">
                <div>
                  <div className="text-xs text-[#8b8b8b]">Сценарий</div>
                  <div className="mt-0.5 text-lg font-medium">Обращение пострадавшего по ОГПО виновника</div>
                  <div className="mt-0.5 font-mono text-xs text-[#8b8b8b]">SC12 · claims</div>
                </div>
                <div className="text-right"><div className="text-xs text-[#8b8b8b]">Уверенность</div><div className="mt-0.5 text-2xl font-medium tabular-nums">90%</div></div>
              </div>
              <div className="mt-3 rounded-lg bg-[#0d0d0d] px-3 py-2 text-sm"><span className="text-[#8b8b8b]">Почему: </span>клиент не виноват, виновник застрахован у нас — это выплата пострадавшему, а не КАСКО и не ДТП сейчас.</div>
              <ul className="mt-4 space-y-1.5 text-sm">
                {CANDS.map((c) => (
                  <li key={c.id} className="grid grid-cols-[minmax(0,1fr)_110px_40px] items-center gap-3">
                    <span className={c.on ? "" : "text-[#8b8b8b]"}>{c.name} <span className="font-mono text-xs text-[#8b8b8b]/70">{c.id}</span></span>
                    <span className="h-1.5 overflow-hidden rounded-full bg-white/[.06]"><span className={`block h-full rounded-full ${c.on ? "bg-s6" : "bg-s4/70"}`} style={{ width: `${c.v * 100}%` }} /></span>
                    <span className="text-right font-mono text-xs tabular-nums text-[#8b8b8b]">{Math.round(c.v * 100)}%</span>
                  </li>
                ))}
              </ul>
            </div>
            <div className="rounded-xl border border-[#242424] bg-[#171717] p-4">
              <div className="flex items-center justify-between"><div><div className="text-sm font-medium">Скорость</div><div className="text-xs text-[#8b8b8b]">от конца реплики до первого звука</div></div><div className="text-lg font-medium tabular-nums text-success-foreground">960 мс</div></div>
              <div className="mt-3 space-y-1.5">
                {SPEED.reduce<{ rows: React.ReactNode[]; left: number }>((acc, r) => {
                  acc.rows.push(
                    <div key={r.k} className="flex items-center gap-2 text-sm">
                      <span className="w-[104px] shrink-0 text-[#8b8b8b]">{r.k}</span>
                      <span className="relative h-2 flex-1 overflow-hidden rounded-full bg-white/[.05]"><span className="absolute inset-y-0 rounded-full bg-s6" style={{ left: `${acc.left}%`, width: `${r.w}%` }} /></span>
                      <span className="w-10 text-right font-mono text-xs tabular-nums text-[#8b8b8b]">{r.v}</span>
                    </div>,
                  );
                  acc.left += r.w;
                  return acc;
                }, { rows: [], left: 0 }).rows}
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
