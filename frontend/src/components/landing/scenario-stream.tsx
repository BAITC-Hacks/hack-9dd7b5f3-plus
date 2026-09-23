import { TextStream } from "@/components/block/text-stream";
import { scenarioLabel } from "@/lib/catalog";
import { Container } from "./section";

const IDS = ["SC01", "SC02", "SC03", "SC06", "SC07", "SC11", "SC12", "SC17", "SC19", "SC27", "SC31", "SC35"];

export function ScenarioStream() {
  const items = IDS.map((id) => `${id} · ${scenarioLabel(id, "ru")}`);
  return (
    <section className="hairline-t hairline-b bg-paper-2 py-6">
      <Container>
        <TextStream
          items={items}
          prefix="Сценарий"
          fontSize="clamp(1rem, 1.6vw, 1.375rem)"
          fontWeight={600}
          height={120}
          className="text-ink tracking-[-0.022em]"
        />
      </Container>
    </section>
  );
}
