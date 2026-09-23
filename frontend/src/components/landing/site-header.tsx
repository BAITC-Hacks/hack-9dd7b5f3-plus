import Link from "next/link";
import { Button } from "@/components/ui/button";
import { BrandMark, Container } from "./section";

export function SiteHeader() {
  return (
    <div className="hairline-b sticky top-0 z-20 bg-paper/90 backdrop-blur-[2px]">
      <Container className="flex h-14 items-center justify-between">
        <Link href="/" className="flex items-center gap-4">
          <BrandMark />
          <span className="marker hidden md:inline">Voice Router · Saqta Insurance</span>
        </Link>
        <nav className="flex items-center gap-2">
          <Button variant="outline" render={<Link href="/call" />} className="hidden sm:inline-flex">
            Симулятор
          </Button>
          <Button variant="outline" render={<Link href="/admin" />} className="hidden sm:inline-flex">
            Консоль
          </Button>
          <Button render={<Link href="/call" />}>Открыть симулятор</Button>
        </nav>
      </Container>
    </div>
  );
}
