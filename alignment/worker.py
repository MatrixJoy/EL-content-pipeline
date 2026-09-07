import argparse
import hashlib
import json
import os
import tempfile
import time
from datetime import datetime, timezone

import boto3
from botocore.exceptions import ClientError
from faster_whisper import WhisperModel

from retry_state import FailureRetry
from timeline import build_timeline, timeline_stats


def env(name, default=None):
    return os.environ.get(name, default)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--limit", type=int, default=1)
    parser.add_argument("--content-id")
    parser.add_argument("--watch", action="store_true")
    parser.add_argument("--poll-seconds", type=int, default=60)
    parser.add_argument("--failure-retry-seconds", type=int, default=int(env("ALIGN_FAILURE_RETRY_SECONDS", "3600")))
    args = parser.parse_args()
    endpoint = ("https" if env("CONTENT_S3_USE_TLS", "false").lower() == "true" else "http") + "://" + env("CONTENT_S3_ENDPOINT", "minio:9000")
    bucket = env("CONTENT_S3_BUCKET", "voa-content-library")
    s3 = boto3.client("s3", endpoint_url=endpoint, aws_access_key_id=env("CONTENT_S3_ACCESS_KEY"), aws_secret_access_key=env("CONTENT_S3_SECRET_KEY"), region_name="us-east-1")
    model_name = env("ALIGN_MODEL", "base.en")
    alignment_version = env("ALIGNMENT_VERSION", "v2")
    model = WhisperModel(model_name, device=env("ALIGN_DEVICE", "cpu"), compute_type=env("ALIGN_COMPUTE_TYPE", "int8"), download_root="/models")
    failures = FailureRetry(args.failure_retry_seconds)
    while True:
        attempted = process_batch(s3, bucket, model, model_name, alignment_version, args, failures)
        if not args.watch:
            break
        print(json.dumps({"event": "alignment_poll_complete", "attempted": attempted, "retry_in_seconds": args.poll_seconds}), flush=True)
        time.sleep(max(5, args.poll_seconds))


def process_batch(s3, bucket, model, model_name, alignment_version, args, failures):
    attempted = 0
    for key in manifest_keys(s3, bucket):
        manifest = read_json(s3, bucket, key)
        content_id = manifest["content_id"]
        if args.content_id and content_id != args.content_id:
            continue
        if not failures.ready(content_id):
            continue
        output_key = f'enrichments/{manifest["source"]}/{content_id}/alignment/{model_name}-{alignment_version}/timeline.json'
        if exists(s3, bucket, output_key):
            continue
        attempted += 1
        try:
            align_one(s3, bucket, manifest, output_key, model, model_name, alignment_version)
            failures.succeeded(content_id)
            print(json.dumps({"event": "timeline_stored", "content_id": content_id, "key": output_key}), flush=True)
        except Exception as error:
            failures.failed(content_id)
            print(json.dumps({"event": "timeline_failed", "content_id": content_id, "error": str(error), "retry_in_seconds": failures.cooldown_seconds}), flush=True)
        if attempted >= args.limit:
            break
    return attempted


def manifest_keys(s3, bucket):
    paginator = s3.get_paginator("list_objects_v2")
    for page in paginator.paginate(Bucket=bucket, Prefix="candidates/"):
        for item in page.get("Contents", []):
            if item["Key"].endswith("/manifest.json") and "/voa/" not in item["Key"]:
                yield item["Key"]


def align_one(s3, bucket, manifest, output_key, model, model_name, alignment_version):
    article = read_json(s3, bucket, manifest["objects"]["article"]["key"])
    audio = s3.get_object(Bucket=bucket, Key=manifest["objects"]["audio"]["key"])["Body"].read()
    with tempfile.NamedTemporaryFile(suffix=".mp3") as file:
        file.write(audio); file.flush()
        segments, info = model.transcribe(file.name, language="en", word_timestamps=True, vad_filter=True, beam_size=5)
        words = []
        for segment in segments:
            for word in segment.words or []:
                words.append({"word": word.word, "start": word.start, "end": word.end, "probability": word.probability})
    sentences = build_timeline(article["paragraphs"], words, int(info.duration * 1000))
    coverage, spoken_sentence_count = timeline_stats(sentences)
    if spoken_sentence_count == 0:
        raise RuntimeError(f'no usable sentence timeline for {manifest["content_id"]}')
    payload = {
        "schema_version": 1,
        "content_id": manifest["content_id"],
        "source": manifest["source"],
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "engine": "faster-whisper",
        "model": model_name,
        "alignment_version": alignment_version,
        "audio_sha256": manifest["objects"]["audio"]["sha256"],
        "article_sha256": manifest["objects"]["article"]["sha256"],
        "duration_ms": int(info.duration * 1000),
        "language": info.language,
        "coverage": coverage,
        "spoken_sentence_count": spoken_sentence_count,
        "sentences": sentences,
    }
    data = json.dumps(payload, ensure_ascii=False, indent=2).encode()
    s3.put_object(Bucket=bucket, Key=output_key, Body=data, ContentType="application/json", Metadata={"sha256": hashlib.sha256(data).hexdigest()})


def read_json(s3, bucket, key):
    return json.loads(s3.get_object(Bucket=bucket, Key=key)["Body"].read())


def exists(s3, bucket, key):
    try:
        s3.head_object(Bucket=bucket, Key=key); return True
    except ClientError as error:
        if error.response["Error"]["Code"] in ("404", "NoSuchKey"): return False
        raise


if __name__ == "__main__":
    main()
