# Official case — Halyk Bank · Voice Router (track: Коммуникации)

> ТЗ: https://docs.google.com/document/d/1e-F3ahQwPSdRMugpLO1vQ0_gFf5q9hIATUt0GxC3bEM · Starter kit: https://drive.google.com/file/d/1sHE56gXnzdscHz5lMcNbwd1VsJIVLFUv (unpacked into /data). Dataset README: data/README.ru.md. Below: RU ТЗ verbatim, then EN.

﻿RU
ТЕХНИЧЕСКОЕ ЗАДАНИЕ


1. Общие сведения
Заказчик: АО Народный Банк Казахстана
Хакатон: HackAlem AI
Трек: Halyk Bank
Название задачи: AI для корпоративных продуктов
Тип задачи: Product-разработка, AI/LLM-engineering, Full-stack
Инструменты: Разрешено использовать любые модели и сервисы: облачные и локальные LLM, STT, TTS.
2. Проблема
Компания ведёт два потока коммуникаций - внутрь, с сотрудниками, и наружу, с клиентами. Оба работают на устаревшей логике.
Внутренний контур 
Карьера сотрудника - это десятки событий: онбординг, обучение, аттестации, менторство, ротации. Они доходят до человека разрозненными уведомлениями с дедлайнами. Сотрудник не видит траектории и не понимает, что ему даст конкретная активность, поэтому проходит всё формально. Компания расходует бюджет на развитие полностью, но получает низкую завершаемость программ и слабую явку на добровольные активности.
Внешний контур
Голосовые роботы научились хорошо говорить и слышать. Но слой, выбирающий сценарий разговора, у большинства систем остаётся классификатором на энкодерной модели — обученным на фиксированных формулировках и ломающимся там, где начинается живая речь: смена темы посреди диалога, запрос между двумя сценариями, переход с русского на казахский внутри фразы. Каждый неверный выбор — перевод на оператора или потерянный клиент.
Общее у двух задач: узкое место не в технологии, а в слое принятия решения. И в обоих случаях его сегодня можно построить на LLM, понимающей контекст, а не сопоставляющей формулировки с обучающей выборкой.
3. Стейкхолдеры
Пользователи: сотрудники компании; HR-специалисты и руководители подразделений; клиенты контакт-центра; операторы и супервизоры.
Ключевые боли:
- Как объяснить сотруднику, зачем ему конкретная активность и куда она ведёт?
- Как сделать профессиональный рост видимым, чтобы он мотивировал?
- Как научить робота понимать клиента, говорящего живым языком?
- Как сделать это, не убив скорость — пауза дольше секунды в разговоре читается как сбой связи.
4. Выбор кейса
- Кейс 1: Career Quest - AI-навигатор развития сотрудника.
- Кейс 2: Voice Router - голосовой робот с AI-слоем выбора сценария


КЕЙС: VOICE ROUTER
1. Название. Voice Router - гибридный голосовой AI-робот с LLM-слоем выбора сценария.
2. Проблема. Клиент контакт-центра сталкивается с роботом, который хорошо говорит и слышит, но выбирает сценарий классификатором на энкодерной модели, из-за чего при смене темы, запросе на стыке сценариев или переходе между русским и казахским разговор уходит не туда: клиент переводится на оператора и в части случаев уходит вовсе.
3. Пользователь. Клиент контакт-центра: говорит своими словами → робот с первой реплики понимает суть и запускает нужный сценарий → при смене темы переключается, не теряя контекст → вопрос закрывается без оператора. Второй пользователь - супервизор: видит, какие сценарии выбирались, где робот сомневался и ошибся.
4. Задача. Разработать голосового AI-робота с веб-интерфейсом симуляции, который заменяет классификатор намерений на LLM-слой, понимающий контекст диалога, и выдаёт голосовой ответ клиенту плюс трассировку для супервизора: какой сценарий выбран, почему, за какое время. 
Оценивается качество маршрутизации, а не качество синтеза и распознавания речи. Скорость — отдельный измеряемый показатель, дающий дополнительные баллы.
5. Вход: Вход: голосовая речь через микрофон, текст как резервный канал. Русский и казахский, включая переключение внутри диалога. Пример: «Здравствуйте, я вчера оплатил, деньги списались, а заказ не подтвердился… а, и ещё, адрес доставки поменять надо». Объём: 40 сценариев, диалог до 10 реплик.
6. Выход: голосовой ответ и панель трассировки — транскрипт, выбранный сценарий с обоснованием, альтернативы, замер задержки по этапам. Допустимая задержка: ориентир по выбору сценария — 500 мс, от конца реплики до начала ответа — 1,5 секунды. Замеры отображаются в интерфейсе и учитываются в баллах, но не являются условием сдачи.
7. Данные и инструменты. Способ доступа: стартовый кит, выдаётся в день хакатона.
Состав:
— scenarios.json — 40 сценариев: назначение, границы с соседними сценариями, параметры, действия, примеры запросов и ответов на русском и казахском
— dialogs_sample.json — 10 размеченных примеров диалогов, включая смену темы и смешанную речь
— knowledge_base.json, mock_backend.json — факты о компании и тестовые клиенты для ответов
— dev_utterances.json и evaluate.py — размеченные реплики и скрипт для самостоятельного замера точности
Использование собственных моделей допускается, но преимуществом при оценке не является: кейс про маршрутизацию, а не про соревнование речевых моделей.
Компания в данных вымышленная (страховая), подробное описание в README стартового кита. 10 проверочных реплик с ожидаемыми сценариями хранятся у жюри и командам не выдаются.
Ограничения: каталог обезличен, реальных клиентских данных не содержит. Расширение собственными сценариями разрешено, оценка идёт по исходным сорока.
8. Must-have
Требование
Проверка
Голосовое взаимодействие в вебе
Жюри говорит в микрофон - робот распознаёт и отвечает голосом. Текст только как дополнение
Выбор сценария на LLM-слое
Команда показывает устройство слоя. Intent-классификатор энкодерного типа не засчитывается
Корректность выбора
Жюри зачитывает 10 одинаковых для всех реплик - простые, со сменой темы, смешанной речью, на стыке сценариев. Считается доля попаданий
Панель трассировки
После каждой реплики виден сценарий, обоснование, альтернативы, время по этапам
Русский и казахский
Две реплики набора на казахском, одна со смешением языков внутри фразы


9. Опционально. Гибридная архитектура — быстрый путь для очевидных запросов, LLM для сложных, с измеримым выигрышем по задержке; достижение ориентира в 500 мс; удержание контекста и возврат к прерванной теме; переспросить вместо угадывания и передать оператору вместе с контекстом; извлечение параметров из речи; потоковая обработка; определение эмоции и подстройка тона; панель супервизора со статистикой ошибок; редактирование каталога без разработчиков.
10. Ограничения.
Нельзя: выбор сценария на готовом intent-классификаторе — ровно та архитектура, от которой уходит кейс. Хардкод сопоставления проверочных реплик со сценариями. Реальные записи разговоров.
Учесть:
— Приватность: данные синтетические; внешние API (LLM, STT, TTS) разрешены, реальные персональные данные в них не передаются
— Безопасность: робот не выполняет необратимых действий без подтверждения клиента
— Explainability: супервизор видит логику, а не голый вердикт
— Инфраструктура: запуск одной командой
— Производительность: задержка измеряется и даёт дополнительные баллы, но не снижает основную оценку
5. Минимальные требования
Решение принимает реальный пользовательский ввод, обрабатывает его с применением LLM в точке содержательного решения, а не для косметической генерации текста, возвращает результат с объяснением и работает на данных, а не на записанном демо-сценарии.
AI помогает человеку, а не подменяет его. В кейсе 1 решение о развитии остаётся за сотрудником и руководителем. В кейсе 2 робот обязан передавать разговор оператору там, где не справляется.
6. Жёсткие ограничения
- AI не является единственным источником истины; где модель не уверена - она это показывает
- Решения должны быть объяснимыми
- Не принимаются: «чёрный ящик»; демо на одном подготовленном сценарии; решения без реальной обработки данных; публичные рейтинги сотрудников (кейс 1); выбор сценария на intent-классификаторе (кейс 2)
7. Что ценится
Продуктовость - пользователь работает без объяснений от разработчика. Объяснимость. Масштабируемость архитектуры. Учёт ограничений домена, а не только технической стороны: психология мотивации в кейсе 1, темп живого разговора в кейсе 2. Честное обозначение границ применимости.
8. Артефакты. Репозиторий: да. README: да. 
________________




Критерий
Что оцениваем
Баллы
Соответствие задаче и работоспособность
Оценивается, насколько решение соответствует поставленной задаче и позволяет реализовать основной заявленный сценарий.
25
Техническая реализация
Оценивается качество технической реализации решения: выбранный подход, архитектура, взаимодействие компонентов, использование AI/agentic AI и других технологий. Учитывается соответствие фактической реализации заявленной логике проекта.
25
README и воспроизводимость
Оценивается, насколько документация позволяет понять устройство проекта, используемые технологии, порядок запуска и основной сценарий работы. Также учитывается возможность воспроизвести и проверить решение на основании материалов репозитория.
25
Ценность и применимость решения
Оценивается, насколько решение отвечает обозначенной проблеме. Учитывается практическая применимость представленного подхода.
15
Потенциал развития и оригинальность подхода
Оценивается потенциал дальнейшего развития решения, его применения в более широком масштабе, а также наличие обоснованных нестандартных или оригинальных подходов к реализации задачи.
10
Итого
—
100



---
EN
TECHNICAL SPECIFICATION
 
1. General Information
Client: Halyk Bank of Kazakhstan JSC
Hackathon: HackAlem AI
Track: Halyk Bank
Task name: AI for Corporate Products
Task type: Product development, AI/LLM engineering, Full-stack
Tools: Any models and services may be used: cloud and local LLMs, STT, TTS.
2. Problem
The company runs two communication streams: internal, with employees, and external, with customers. Both run on outdated logic.
Internal Loop
An employee's career consists of dozens of events: onboarding, training, performance reviews, mentoring, rotations. They reach the person as scattered notifications with deadlines. The employee doesn't see a trajectory and doesn't understand what a given activity will give them, so they go through everything as a formality. The company spends its full development budget but gets low program completion rates and weak turnout for voluntary activities.
External Loop
Voice bots have learned to speak and listen well. But in most systems, the layer that selects the conversation scenario is still an encoder-based classifier: trained on fixed phrasings and breaking down where natural speech begins, such as a topic change mid-dialogue, a request that falls between two scenarios, or a switch from Russian to Kazakh within a single phrase. Every wrong choice means a transfer to an operator or a lost customer.
What the two problems have in common: the bottleneck is not the technology but the decision-making layer. In both cases, it can now be built on an LLM that understands context rather than matching phrasings against a training set.
3. Stakeholders
Users: company employees; HR specialists and department heads; contact center customers; operators and supervisors.
Key pain points:
- How do you explain to an employee why a specific activity matters to them and where it leads?
- How do you make professional growth visible so that it motivates?
- How do you teach a bot to understand a customer who speaks naturally?
- How do you do this without killing speed? A pause longer than one second in a conversation reads as a connection failure.
4. Case Selection
- Case 1: Career Quest - an AI navigator for employee development.
- Case 2: Voice Router - a voice bot with an AI scenario-selection layer
 
CASE: VOICE ROUTER
1)       Name. Voice Router - a hybrid voice AI bot with an LLM scenario-selection layer.
2)       Problem. A contact center customer encounters a bot that speaks and listens well but selects scenarios with an encoder-based classifier. As a result, when the topic changes, when a request falls at the boundary between scenarios, or when the customer switches between Russian and Kazakh, the conversation goes off track: the customer is transferred to an operator and, in some cases, leaves altogether.
3)       User. Contact center customer: speaks in their own words → from the first utterance, the bot grasps the point and launches the right scenario → when the topic changes, it switches without losing context → the issue is resolved without an operator. The second user is the supervisor, who sees which scenarios were selected and where the bot was uncertain or made mistakes.
4)       Task. Develop a voice AI bot with a web simulation interface that replaces the intent classifier with an LLM layer that understands dialogue context, and provides a voice response to the customer plus a trace for the supervisor: which scenario was selected, why, and how long it took.
Routing quality is evaluated, not the quality of speech synthesis or recognition. Speed is a separate measured metric that earns bonus points.
5)       Input: voice speech via microphone, with text as a fallback channel. Russian and Kazakh, including switching within a dialogue. Example: "Hello, I paid yesterday, the money was debited, but the order wasn't confirmed… oh, and also, I need to change the delivery address." Scope: 40 scenarios, dialogues of up to 10 turns.
6)       Output: voice response and a trace panel showing the transcript, the selected scenario with its rationale, alternatives, and latency measured per stage. Acceptable latency: the target for scenario selection is 500 ms; from the end of the utterance to the start of the response, 1.5 seconds. Measurements are displayed in the interface and count toward the score, but are not a submission requirement.
7)       Data and Tools. Access: starter kit, provided on the day of the hackathon.
Contents:
— scenarios.json — 40 scenarios: purpose, boundaries with neighboring scenarios, parameters, actions, sample requests and responses in Russian and Kazakh
— dialogs_sample.json — 10 annotated sample dialogues, including topic changes and mixed-language speech
— knowledge_base.json, mock_backend.json — company facts and test customers for responses
— dev_utterances.json and evaluate.py — annotated utterances and a script for measuring accuracy on your own
Using your own models is allowed but gives no advantage in evaluation: this case is about routing, not a competition between speech models.
The company in the data is fictional (an insurance company); a detailed description is in the starter kit README. The 10 test utterances with expected scenarios are held by the jury and are not given to teams.
Constraints: the catalog is anonymized and contains no real customer data. Extending it with your own scenarios is allowed; evaluation is based on the original forty.
8)       Must-have
Requirement
Verification
Voice interaction on the web
The jury speaks into the microphone - the bot recognizes the speech and responds by voice. Text only as a supplement
Scenario selection via an LLM layer
The team demonstrates how the layer works. An encoder-type intent classifier does not count
Selection accuracy
The jury reads the same 10 utterances for all teams - simple ones, with topic changes, mixed-language speech, and at scenario boundaries. The hit rate is calculated
Trace panel
After each utterance, the scenario, rationale, alternatives, and per-stage timing are visible
Russian and Kazakh
Two utterances in the set are in Kazakh, one mixes languages within a phrase
 
9)          Optional. Hybrid architecture: a fast path for obvious requests and an LLM for complex ones, with a measurable latency gain; reaching the 500 ms target; retaining context and returning to an interrupted topic; asking for clarification instead of guessing and handing off to an operator along with the context; extracting parameters from speech; streaming processing; emotion detection and tone adjustment; a supervisor panel with error statistics; catalog editing without developers.
10)   Constraints.
Not allowed: scenario selection using an off-the-shelf intent classifier, which is exactly the architecture this case is moving away from. Hardcoding the mapping of test utterances to scenarios. Real call recordings.
Take into account:
— Privacy: the data is synthetic; external APIs (LLM, STT, TTS) are allowed, and no real personal data is sent to them
— Safety: the bot does not perform irreversible actions without customer confirmation
— Explainability: the supervisor sees the reasoning, not just a bare verdict
— Infrastructure: launch with a single command
— Performance: latency is measured and earns bonus points but does not lower the main score
5. Minimum Requirements
The solution accepts real user input, processes it using an LLM at the point of the substantive decision (not for cosmetic text generation), returns the result with an explanation, and runs on data rather than a pre-recorded demo scenario.
AI helps people rather than replacing them. In Case 1, development decisions remain with the employee and their manager. In Case 2, the bot must hand the conversation over to an operator wherever it cannot cope.
6. Hard Constraints
- AI is not the sole source of truth; where the model is uncertain, it shows this
- Decisions must be explainable
- Not accepted: a "black box"; a demo built on a single prepared scenario; solutions without real data processing; public employee rankings (Case 1); scenario selection via an intent classifier (Case 2)
7. What Is Valued
Product quality - the user can work without explanations from the developer. Explainability. Architecture scalability. Accounting for domain constraints, not just the technical side: the psychology of motivation in Case 1, the pace of live conversation in Case 2. Honest disclosure of the limits of applicability.
8. Deliverables. Repository: yes. README: yes.
________________




Criterion
What is evaluated
Points
Compliance with the task and functionality
The assessment considers how well the solution meets the assigned task and enables the main stated scenario to be implemented.
25
Technical implementation
The quality of the solution’s technical implementation is assessed, including the selected approach, architecture, interaction between components, and use of AI/agentic AI and other technologies. The consistency of the actual implementation with the stated project logic is taken into account.
25
README and reproducibility
The assessment considers whether the documentation makes it possible to understand the project structure, technologies used, launch procedure, and main operating scenario. The possibility of reproducing and checking the solution based on the repository materials is also taken into account.
25
Value and applicability of the solution
The assessment considers how well the solution addresses the stated problem. The practical applicability of the proposed approach is taken into account.
15
Development potential and originality of the approach
The assessment considers the potential for further development of the solution and its use at a broader scale, as well as the presence of well-founded unconventional or original approaches to implementing the task.
10
Total
—
100