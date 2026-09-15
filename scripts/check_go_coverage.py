"""Count statements from the Go coverage profile, including untested files."""
import sys
from pathlib import Path

total = covered = 0
for line in Path(sys.argv[1]).read_text().splitlines()[1:]:
    _, statements, count = line.rsplit(' ', 2)
    total += int(statements)
    if int(count):
        covered += int(statements)
percent = 100 * covered / total if total else 0
print(f'Backend statement coverage: {percent:.2f}% (required: 80%)')
sys.exit(0 if percent >= 80 else 1)
