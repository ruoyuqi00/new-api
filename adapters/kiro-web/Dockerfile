FROM python:3.12-alpine

WORKDIR /app
COPY kiro_web_adapter.py /app/kiro_web_adapter.py

ENV KIRO_CREDENTIALS_FILE=/config/credentials.json
ENV KIRO_ADAPTER_API_KEY_FILE=/config/generated-kiro-api-key.txt
ENV KIRO_ADAPTER_HOST=0.0.0.0
ENV KIRO_ADAPTER_PORT=8991

EXPOSE 8991
CMD ["python3", "/app/kiro_web_adapter.py"]
