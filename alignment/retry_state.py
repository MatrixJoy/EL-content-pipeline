import time


class FailureRetry:
    def __init__(self, cooldown_seconds, clock=time.monotonic):
        self.cooldown_seconds = max(5, cooldown_seconds)
        self.clock = clock
        self.deadlines = {}

    def ready(self, content_id):
        deadline = self.deadlines.get(content_id)
        if deadline is None:
            return True
        if self.clock() < deadline:
            return False
        self.deadlines.pop(content_id, None)
        return True

    def failed(self, content_id):
        deadline = self.clock() + self.cooldown_seconds
        self.deadlines[content_id] = deadline
        return deadline

    def succeeded(self, content_id):
        self.deadlines.pop(content_id, None)
