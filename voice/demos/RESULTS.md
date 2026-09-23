# Voice demo results

Measured with `go run ./cmd/voicedemo` on the team ElevenLabs key. Regenerate: `go run ./cmd/voicedemo tts -force && go run ./cmd/voicedemo stt`.

## Text-to-speech (WebSocket streaming)

| Demo | Lang | Model | Connect ms | First audio ms | Total ms | Audio ms | Text |
|---|---|---|---:|---:|---:|---:|---|
| [01_ru_greeting](01_ru_greeting.wav) | ru | `eleven_flash_v2_5` | 226 | **245** | 350 | 3947 | Здравствуйте! Я голосовой помощник Saqta. Чем могу помочь? |
| [02_kk_greeting](02_kk_greeting.wav) | kk | `eleven_v3_conversational` | 198 | **214** | 1379 | 4800 | Сәлеметсіз бе! Мен Saqta дауыстық көмекшісімін. Қалай көмектесе аламын? |
| [03_mixed_caller](03_mixed_caller.wav) | mixed | `eleven_v3_conversational` | 190 | **221** | 1052 | 3600 | Сәлеметсіз бе, кеше аварияға түстім, но я не виноват. |

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
