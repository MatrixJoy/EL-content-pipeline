import difflib
import re

TOKEN_RE = re.compile(r"[a-z0-9]+(?:'[a-z0-9]+)?")
SENTENCE_RE = re.compile(r".+?(?:[.!?](?=\s|$)|$)", re.DOTALL)


def normalize_tokens(text):
    return TOKEN_RE.findall(text.lower().replace("’", "'"))


def split_sentences(paragraphs):
    sentences = []
    article_tokens = []
    for paragraph in paragraphs:
        text = paragraph["text"].strip()
        for match in SENTENCE_RE.finditer(text):
            sentence = " ".join(match.group(0).split())
            tokens = normalize_tokens(sentence)
            if not tokens:
                continue
            start = len(article_tokens)
            article_tokens.extend(tokens)
            sentences.append({
                "paragraph_index": paragraph["index"],
                "sentence_index": len(sentences),
                "text": sentence,
                "token_start": start,
                "token_end": len(article_tokens),
            })
    return sentences, article_tokens


def build_timeline(paragraphs, asr_words, duration_ms):
    sentences, article_tokens = split_sentences(paragraphs)
    spoken_tokens, spoken_words = [], []
    for word in asr_words:
        tokens = normalize_tokens(word["word"])
        for token in tokens:
            spoken_tokens.append(token)
            spoken_words.append(word)
    matcher = difflib.SequenceMatcher(None, article_tokens, spoken_tokens, autojunk=False)
    article_to_spoken = {}
    for block in matcher.get_matching_blocks():
        for offset in range(block.size):
            article_to_spoken[block.a + offset] = block.b + offset

    for sentence in sentences:
        indexes = [article_to_spoken[i] for i in range(sentence["token_start"], sentence["token_end"]) if i in article_to_spoken]
        token_count = sentence["token_end"] - sentence["token_start"]
        sentence["confidence"] = round(len(indexes) / token_count, 4)
        if indexes:
            words = [spoken_words[i] for i in indexes]
            sentence["start_ms"] = int(min(w["start"] for w in words) * 1000)
            sentence["end_ms"] = int(max(w["end"] for w in words) * 1000)
        else:
            sentence["start_ms"] = None
            sentence["end_ms"] = None
    previous = 0
    for sentence in sentences:
        sentence["spoken"] = sentence["confidence"] >= 0.5
        if not sentence["spoken"]:
            sentence["start_ms"] = None
            sentence["end_ms"] = None
            continue
        latest_start = max(0, duration_ms - 1)
        sentence["start_ms"] = max(previous, min(latest_start, sentence["start_ms"]))
        sentence["end_ms"] = min(duration_ms, max(sentence["start_ms"] + 1, sentence["end_ms"]))
        previous = sentence["end_ms"]
    return sentences
