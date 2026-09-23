FROM python:3.13-slim
WORKDIR /app
COPY router.py server.py prompt.md scenarios.index.txt ./
ENV CORE_LLM_HOST=0.0.0.0 CORE_LLM_PORT=8090 PYTHONDONTWRITEBYTECODE=1
USER nobody
EXPOSE 8090
CMD ["python", "server.py"]
