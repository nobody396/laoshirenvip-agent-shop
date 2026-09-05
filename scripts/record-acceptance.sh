#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
output_dir="$root/output/acceptance-recordings"
pid_file="$output_dir/recording.pid"
current_file="$output_dir/current-file.txt"
mkdir -p "$output_dir"

case "${1:-status}" in
  start)
    if [[ -s "$pid_file" ]] && kill -0 "$(cat "$pid_file")" 2>/dev/null; then
      echo "recording already active"
      exit 1
    fi
    stamp=$(TZ=Asia/Shanghai date '+%Y%m%d-%H%M%S-CST')
    target="$output_dir/gpt-go-full-flow-$stamp.mov"
    nohup ffmpeg -hide_banner -loglevel warning \
      -f avfoundation -framerate 20 -capture_cursor 1 -i '6:none' \
      -c:v h264_videotoolbox -b:v 5M -movflags +faststart "$target" \
      >"$output_dir/ffmpeg.log" 2>&1 &
    echo $! >"$pid_file"
    printf '%s\n' "$target" >"$current_file"
    sleep 1
    kill -0 "$(cat "$pid_file")"
    echo "recording=$target"
    ;;
  stop)
    [[ -s "$pid_file" ]] || { echo "no active recording"; exit 1; }
    pid=$(cat "$pid_file")
    if kill -0 "$pid" 2>/dev/null; then
      kill -INT "$pid"
      for _ in {1..30}; do
        kill -0 "$pid" 2>/dev/null || break
        sleep 1
      done
    fi
    target=$(cat "$current_file")
    : >"$pid_file"
    ffprobe -v error -show_entries format=duration,size \
      -show_entries stream=codec_name,width,height -of json "$target"
    echo "recording=$target"
    ;;
  status)
    if [[ -s "$pid_file" ]] && kill -0 "$(cat "$pid_file")" 2>/dev/null; then
      echo "active pid=$(cat "$pid_file") file=$(cat "$current_file")"
    else
      echo "inactive"
    fi
    ;;
  *)
    echo "usage: $0 {start|stop|status}" >&2
    exit 2
    ;;
esac
