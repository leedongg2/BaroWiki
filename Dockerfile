# ============================================================
#  바로위키 (openNAMU 확장) 컨테이너
#
#  SQLite 드라이버가 순수 Go(modernc.org/sqlite)라 CGO 가 필요 없다.
#  화면(views)과 번역표(lang)는 실행 파일 안에 들어간다.
#
#  Fly.io · Railway · Render · 직접 쓰는 VPS 어디서나 그대로 돈다.
# ============================================================

# ---------- 1단계 : 빌드 ----------
FROM golang:1.25-alpine AS build

WORKDIR /src

# 의존성만 먼저 받아 두면 소스만 바뀔 때 다시 안 받는다
COPY go.mod ./
COPY go.su[m] ./
RUN go mod download

COPY . .

# 정적 실행 파일 하나로 만든다
RUN CGO_ENABLED=0 GOOS=linux go build \
      -trimpath -ldflags="-s -w" \
      -o /out/onamu .


# ---------- 2단계 : 실행 ----------
FROM alpine:3.20

# ca-certificates : 링크 검사 · 보존본 확인 같은 바깥 HTTPS 호출에 필요
# tzdata          : 위키가 지역 시간을 쓴다 (기록 시각이 어긋나지 않게)
RUN apk add --no-cache ca-certificates tzdata wget

ENV TZ=Asia/Seoul

WORKDIR /app

COPY --from=build /out/onamu /app/onamu

# 위키가 쓰는 것은 이 폴더 하나뿐이다 (DB · 올린 파일 · 설정)
#   data/wiki.db      SQLite 본체 (+ -wal, -shm)
#   data/images/      올린 파일
#   data/set.json     DB 설정
RUN mkdir -p /app/data/images

# DB 파일을 data 폴더 안에 두어 볼륨 하나로 전부 보존한다
ENV NAMU_DB=data/wiki
ENV NAMU_DB_TYPE=sqlite

VOLUME ["/app/data"]

EXPOSE 3000

# 살아 있는지 확인
#   실제 DB 연결과 초기화 상태까지 확인한다.
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
  CMD wget --spider -q http://127.0.0.1:3000/readyz || exit 1

# 첫 인자가 포트다
CMD ["/app/onamu", "3000"]
