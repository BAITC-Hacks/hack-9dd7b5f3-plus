# Voice demo results

Measured with `go run ./cmd/voicedemo` on the team ElevenLabs key. Regenerate: `go run ./cmd/voicedemo tts -force && go run ./cmd/voicedemo stt && go run ./cmd/voicedemo e2e`.

## End to end: caller audio -> STT -> brain -> TTS (phone-like VAD mode)

| Input | Transcript | Lang in/out | Scenario | Reply (audio) | STT final ms | LLM first text ms | TTS first audio ms | **End of speech -> reply audio ms** | First sound ms (incl. filler) | Speculative |
|---|---|---|---|---|---:|---:|---:|---:|---:|---|
| [03_mixed_caller](03_mixed_caller.wav) | Сәлеметсіз бе? Іші аварияға түстім, но я не виноват. | mixed/kk | SC12 0.75 | [Өкінішке орай, аварияға ұшырағаныңызды естігеніме өкініштімін. Сіздің полисіңіз бойынша өтемақы рәсімдеуге көмектесейін. Кінәлі көліктің мемлекеттік нөмірін және оқиға болған күнді айтыңызшы.](03_mixed_caller_reply.wav) | 837 | 732 | 180 | **1950** | 1501 | false |
| [04_ru_caller](04_ru_caller.wav) | Здравствуйте! Я вчера оплатил полис, деньги списались, а полис не пришел. | ru/ru | SC30 0.95 | [Здравствуйте! Разберёмся. Когда был платёж и на какую сумму?](04_ru_caller_reply.wav) | 1076 | 531 | 234 | **1703** | 1703 | true |
| [05_kk_caller](05_kk_caller.wav) | Сәлеметсіз бе! Тулысымның мерзімін ұзарқым келеді. | kk/kk | SC27 0.95 | [Сәлеметсіз бе! Ұзартайық. Полисіңіздің нөмірін айтыңызшы.](05_kk_caller_reply.wav) | 1087 | 495 | 206 | **1916** | 1501 | false |

Brain: built-in OpenRouter agent (`google/gemini-2.5-flash-lite`) with the Saqta catalog. *End of speech* = end of the caller's last word (Scribe word timestamps), so the STT column includes the server VAD silence window (phone mode); push-to-talk on the web removes that window (see the STT table). *First sound* includes the cached filler ("Секунду.") that covers slow turns. *Speculative* = the reply was generated from a stable partial transcript during the VAD window and released when the final transcript matched.

## Text-to-speech (WebSocket streaming)

| Demo | Lang | Model | Connect ms | First audio ms | Total ms | Audio ms | Text |
|---|---|---|---:|---:|---:|---:|---|
| [01_ru_greeting](01_ru_greeting.wav) | ru | `eleven_flash_v2_5` | 226 | **245** | 350 | 3947 | Здравствуйте! Я голосовой помощник Saqta. Чем могу помочь? |
| [02_kk_greeting](02_kk_greeting.wav) | kk | `eleven_v3_conversational` | 198 | **214** | 1379 | 4800 | Сәлеметсіз бе! Мен Saqta дауыстық көмекшісімін. Қалай көмектесе аламын? |
| [03_mixed_caller](03_mixed_caller.wav) | mixed | `eleven_v3_conversational` | 190 | **221** | 1052 | 3600 | Сәлеметсіз бе, кеше аварияға түстім, но я не виноват. |
| [04_ru_caller](04_ru_caller.wav) | ru | `eleven_flash_v2_5` | 273 | **211** | 360 | 4040 | Здравствуйте, я вчера оплатил полис, деньги списались, а полис не пришёл. |
| [05_kk_caller](05_kk_caller.wav) | kk | `eleven_v3_conversational` | 199 | **208** | 1058 | 3200 | Сәлеметсіз бе, полисімнің мерзімін ұзартқым келеді. |

*First audio* = first text sent -> first audio chunk received (what the caller waits for after the LLM starts talking). The socket is opened in advance during the call, so *Connect* is hidden.

## Speech-to-text (Scribe v2 Realtime, audio streamed at real-time pace)

| Input | Mode | Audio ms | First partial ms | Final after audio end ms | Final after last word ms | Lang | Transcript | Expected |
|---|---|---:|---:|---:|---:|---|---|---|
| 01_ru_greeting | manual | 3947 | 2166 | **271** | 551 | ru | Здравствуйте! Я голосовой помощник Сакта. Чем могу помочь? | Здравствуйте! Я голосовой помощник Saqta. Чем могу помочь? |
| 01_ru_greeting | vad | 3947 | 2314 | **778** | 1059 | ru | Здравствуйте! Я голосовой помощник Сакта. Чем могу помочь? | Здравствуйте! Я голосовой помощник Saqta. Чем могу помочь? |
| 02_kk_greeting | manual | 4800 | 2213 | **263** | 523 | kk | Сәлеметсіз бе! Мен Сақта дауыстық көмекшісімін. Қалай көмектесе аламын? | Сәлеметсіз бе! Мен Saqta дауыстық көмекшісімін. Қалай көмектесе аламын? |
| 02_kk_greeting | vad | 4800 | 2416 | **729** | 989 | kk | Сәлеметсіз бе! Мен Сақта дауыстық көмекшісімін. Қалай көмектесе аламын? | Сәлеметсіз бе! Мен Saqta дауыстық көмекшісімін. Қалай көмектесе аламын? |
| 03_mixed_caller | manual | 3600 | 2320 | **325** | 725 | kk | Сәлеметсіз бе? Іші аварияға түстім, но я не виноват. | Сәлеметсіз бе, кеше аварияға түстім, но я не виноват. |
| 03_mixed_caller | vad | 3600 | 2399 | **577** | 977 | kk | Сәлеметсіз бе? Іші аварияға түстім, но я не виноват. | Сәлеметсіз бе, кеше аварияға түстім, но я не виноват. |

*vad* = server end-of-turn detection (phone, hands-free; includes the configured silence window). *manual* = push-to-talk: the client commits on button release.
