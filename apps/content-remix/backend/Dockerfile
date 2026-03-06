FROM python:3.12-slim

ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1

WORKDIR /app

COPY apps/content-remix/backend/requirements.txt /tmp/requirements.txt
RUN pip install --no-cache-dir -r /tmp/requirements.txt

COPY apps/content-remix/backend /app

EXPOSE 18061

CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "18061"]

