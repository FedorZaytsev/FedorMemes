
BUILDTIME=$(date -u +%Y-%m-%d.%H:%M:%S)
GOOS=linux GOARCH=amd64 CC=x86_64-w64-mingw32-gcc CGO_ENABLED=1 vgo build -ldflags="-w -X main.Version=$1 -X main.BuildTime=$BUILDTIME"
