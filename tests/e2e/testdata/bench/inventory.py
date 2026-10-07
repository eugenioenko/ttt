"""Inventory reconciliation for warehouse snapshots."""

from __future__ import annotations

import csv
import logging
from dataclasses import dataclass, field
from pathlib import Path
from typing import Iterable, Iterator

log = logging.getLogger(__name__)

DEFAULT_THRESHOLD = 0.05  # tolerated relative drift


@dataclass(frozen=True)
class Item:
    sku: str
    quantity: int
    location: str = "main"
    tags: tuple[str, ...] = field(default_factory=tuple)

    @property
    def is_empty(self) -> bool:
        return self.quantity <= 0


class Snapshot:
    """A point-in-time view of stock levels."""

    def __init__(self, items: Iterable[Item]) -> None:
        self._items = {item.sku: item for item in items}

    def __len__(self) -> int:
        return len(self._items)

    def __iter__(self) -> Iterator[Item]:
        yield from sorted(self._items.values(), key=lambda i: (i.location, i.sku))

    @classmethod
    def from_csv(cls, path: Path) -> "Snapshot":
        with path.open(newline="", encoding="utf-8") as handle:
            rows = csv.DictReader(handle)
            return cls(Item(r["sku"], int(r["qty"]), r.get("loc", "main")) for r in rows)

    def drift(self, other: "Snapshot", threshold: float = DEFAULT_THRESHOLD) -> dict[str, float]:
        report: dict[str, float] = {}
        for sku, item in self._items.items():
            previous = other._items.get(sku)
            if previous is None or previous.quantity == 0:
                continue
            change = abs(item.quantity - previous.quantity) / previous.quantity
            if change > threshold:
                report[sku] = round(change, 4)
                log.warning("sku %s drifted by %.1f%%", sku, change * 100)
        return report


if __name__ == "__main__":
    before = Snapshot.from_csv(Path("before.csv"))
    after = Snapshot.from_csv(Path("after.csv"))
    print(f"{len(after.drift(before))} items drifted beyond {DEFAULT_THRESHOLD:.0%}")
