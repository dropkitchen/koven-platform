FROM debian:stable-slim

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y \
    build-essential \
    cmake \
    pkg-config \
    libpaho-mqtt-dev \
    libpaho-mqtt1.3 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY koven /app
RUN mkdir -p build && \
    cd build && \
    cmake .. && \
    make

CMD ["./build/koven"]
