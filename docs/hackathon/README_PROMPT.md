# README prompt (from organizers' participant instructions)
> If the case document contains its own README prompt — use that one instead. Run at ~17:15 in Codex / Claude Code, then manually add env vars + dependencies (required by rules §5.4.15) and verify every command from a clean clone.

«Проанализируй текущий проект и создай для него полноценный README.md на русском языке.
README должен быть понятен жюри хакатона и содержать:
1. Название проекта
2. Краткое описание — какую проблему решает проект и для кого.
3. Что реализовано — основные функции и возможности решения.
4. Как работает решение — кратко опиши основной пользовательский сценарий от входных данных до результата.
5. Технологии — языки, фреймворки, библиотеки, AI-модели, API и внешние сервисы.
6. Архитектура проекта — основные компоненты и как они взаимодействуют.
7. Установка и запуск — пошаговая инструкция с необходимыми командами.
8. Как проверить решение — пример сценария, который может повторить жюри.
9. Данные и интеграции — какие источники данных, API или внешние сервисы используются.
10. Ограничения — что не реализовано или какие ограничения есть у текущей версии.
11. Ссылка на deployed-версию, если она существует.
Используй только информацию, которую можно подтвердить по текущему репозиторию. Не придумывай функции, технологии или результаты, которых в проекте нет. Оформи README аккуратно в Markdown.»

> ⚠️ The organizers' prompt asks for the final README **in Russian**. Our working README is in English during the build; decide by ~17:15 whether to translate it (or ship RU + EN).

English translation of the prompt (for reference; use the original above when generating):
> "Analyze the current project and create a complete README.md for it in Russian. The README must be clear to the hackathon jury and contain: 1. Project name. 2. Short description — what problem it solves and for whom. 3. What is implemented — main functions and capabilities. 4. How the solution works — the main user scenario from input to result. 5. Technologies — languages, frameworks, libraries, AI models, APIs and external services. 6. Project architecture — main components and how they interact. 7. Installation and launch — step-by-step with the required commands. 8. How to verify — an example scenario the jury can repeat. 9. Data and integrations — data sources, APIs or external services used. 10. Limitations — what is not implemented or what limits the current version has. 11. Link to the deployed version, if any. Use only information that can be confirmed from the current repository. Do not invent functions, technologies or results that the project does not have. Format the README neatly in Markdown."

Additionally required by the regulations (5.4.15 / 5.6.4) — make sure they're present: dependencies & system requirements, environment variables (table), demo access without personal accounts (mock mode / test credentials).
