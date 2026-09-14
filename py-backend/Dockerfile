# DBAWORK py-backend 镜像（gRPC + FastAPI 双进程）
FROM python:3.13-slim
WORKDIR /app
COPY requirements.txt requirements-optional.txt ./
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
ENV DBAWORK_DATA_DIR=/data \
    PYTHONUNBUFFERED=1
EXPOSE 50051 8000
CMD ["bash", "scripts/start.sh"]
