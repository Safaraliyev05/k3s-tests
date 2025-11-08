FROM python:3.10-slim

RUN groupadd -g 3000 appgroup && \
    useradd -r -u 1000 -g appgroup appuser

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

RUN chown -R appuser:appgroup /app

USER appuser

EXPOSE 8080

CMD ["python", "main.py"]
