package router

import (
	"strings"
	"testing"
)

func TestStreamParser(t *testing.T) {
	full := `{"language":"ru","scenarios":[{"id":"SC30","confidence":0.86,"reason":"деньги списаны, полис не оформлен"},{"id":"SC29","confidence":0.8,"reason":"сменить адрес"}],"alternatives":[{"id":"SC26","confidence":0.3}],"is_continuation":false,"slots":{"payment_date":"2026-09-30"},"actions":[{"name":"find_client","args":{"phone":"+77010000003"}}],"handoff":null,"reply":"Понимаю, сейчас разберёмся с платежом. Скажите \"да\", если номер верный: +7 701 000 00 03?"}`
	p := NewStreamParser()
	var decided *Decision
	var reply strings.Builder
	// feed in small chunks to exercise partial decoding
	for i := 0; i < len(full); i += 7 {
		end := i + 7
		if end > len(full) {
			end = len(full)
		}
		d, r := p.Feed(full[i:end])
		if d != nil {
			decided = d
		}
		reply.WriteString(r)
	}
	if decided == nil || decided.Primary() != "SC30" || len(decided.Scenarios) != 2 {
		t.Fatalf("decision not streamed: %+v", decided)
	}
	want := `Понимаю, сейчас разберёмся с платежом. Скажите "да", если номер верный: +7 701 000 00 03?`
	if reply.String() != want {
		t.Fatalf("reply mismatch:\n%s\n%s", reply.String(), want)
	}
	d, repaired, err := p.Finish()
	if err != nil || repaired || d.Reply != want || d.Actions[0].Name != "find_client" {
		t.Fatalf("finish: %v %v %+v", err, repaired, d)
	}
}

func TestParserTruncatedByStop(t *testing.T) {
	trunc := `{"language":"kk","scenarios":[{"id":"SC25","confidence":0.9,"reason":"полис мерзімі"}],"alternatives":[],"is_continuation":false,"slots":{},"actions":[],"handoff":null,`
	p := NewStreamParser()
	p.Feed(trunc)
	d, repaired, err := p.Finish()
	if err != nil || !repaired || d.Primary() != "SC25" {
		t.Fatalf("truncated: %v %v %+v", err, repaired, d)
	}
}

func TestParserFenced(t *testing.T) {
	p := NewStreamParser()
	p.Feed("```json\n{\"language\":\"ru\",\"scenarios\":[{\"id\":\"SC11\",\"confidence\":0.95}],\"alternatives\":[],\"is_continuation\":false,\"slots\":{},\"actions\":[],\"handoff\":null,\"reply\":\"Все целы?\"}\n```")
	d, _, err := p.Finish()
	if err != nil || d.Primary() != "SC11" || d.Reply != "Все целы?" {
		t.Fatalf("fenced: %v %+v", err, d)
	}
}
