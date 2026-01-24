FROM python:3.11-slim

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends git curl build-essential \
    && rm -rf /var/lib/apt/lists/*

RUN pip install --no-cache-dir flask watchdog redis tree-sitter tree-sitter-language-pack pyahocorasick anthropic

# Copy from src context (set in docker-compose)
COPY index/indexer.py .
COPY ranking /app/ranking
COPY domains /app/domains
COPY session /app/session
# Config is mounted via docker-compose: ./config:/app/config

ENV CODEBASE_ROOT=/codebase
ENV PORT=9999

EXPOSE 9999

CMD ["python", "indexer.py"]
