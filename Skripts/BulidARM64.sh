docker buildx build   --platform linux/arm64   -t yaxbase:arm64   --load .

docker save -o builds/yaxbase-arm64.tar yaxbase:arm64