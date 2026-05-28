FROM python:3.12-alpine

WORKDIR /app
COPY requirements.txt /app/requirements.txt
RUN pip install --no-cache-dir -r /app/requirements.txt

COPY relay.py /app/relay.py

ENV MAIL_RELAY_HOST=0.0.0.0
ENV MAIL_RELAY_PORT=1025
ENV MAIL_RELAY_HEALTH_PORT=8080
ENV MAIL_RELAY_PROVIDER=resend
ENV MAIL_RELAY_TIMEOUT_SECONDS=20

EXPOSE 1025 8080
CMD ["python3", "/app/relay.py"]
