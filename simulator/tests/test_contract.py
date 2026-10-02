"""Tests de contrato: los fixtures de schema/ contra el JSON Schema v1."""

import json
from pathlib import Path

import pytest
from jsonschema import Draft202012Validator, FormatChecker
from referencing import Registry, Resource

SCHEMA_DIR = Path(__file__).resolve().parents[2] / "schema"
FIXTURES = SCHEMA_DIR / "fixtures"
SCHEMA = json.loads((SCHEMA_DIR / "event.v1.schema.json").read_text(encoding="utf-8"))
REGISTRY = Registry().with_resource(SCHEMA["$id"], Resource.from_contents(SCHEMA))

EVENT_TYPES = [
    "login_attempt",
    "account_created",
    "cart_action",
    "transaction",
    "read",
    "transaction_outcome",
]


def validator(fragment: str = "") -> Draft202012Validator:
    return Draft202012Validator(
        {"$ref": SCHEMA["$id"] + fragment},
        registry=REGISTRY,
        format_checker=FormatChecker(),
    )


CLIENT = validator("#/$defs/ClientEvent")
EVENT = validator()


def load(path: Path) -> dict:
    return json.loads(path.read_text(encoding="utf-8"))


def fixtures(subdir: str) -> list[Path]:
    return sorted((FIXTURES / subdir).glob("*.json"))


def test_schema_is_valid_draft_2020_12():
    Draft202012Validator.check_schema(SCHEMA)


def test_every_event_type_has_a_valid_fixture():
    for subdir in ("valid/client", "valid/event"):
        assert sorted(p.stem for p in fixtures(subdir)) == sorted(EVENT_TYPES)


@pytest.mark.parametrize("path", fixtures("valid/client"), ids=lambda p: p.stem)
def test_valid_client_event(path):
    assert list(CLIENT.iter_errors(load(path))) == []


@pytest.mark.parametrize("path", fixtures("valid/event"), ids=lambda p: p.stem)
def test_valid_event(path):
    assert list(EVENT.iter_errors(load(path))) == []


@pytest.mark.parametrize("path", fixtures("invalid/client"), ids=lambda p: p.stem)
def test_invalid_client_event(path):
    assert not CLIENT.is_valid(load(path))


@pytest.mark.parametrize("path", fixtures("invalid/event"), ids=lambda p: p.stem)
def test_invalid_event(path):
    assert not EVENT.is_valid(load(path))


def test_client_event_is_not_a_complete_event():
    """Sin los campos del Gateway, un evento de cliente no es un evento persistido."""
    client = load(FIXTURES / "valid" / "client" / "login_attempt.json")
    assert not EVENT.is_valid(client)


def test_fraud_labels_are_rejected():
    """ADR-004: ninguna etiqueta de fraude puede viajar en el evento."""
    client = load(FIXTURES / "valid" / "client" / "login_attempt.json")
    for label in ("is_fraud", "fraud_type", "label"):
        assert not CLIENT.is_valid({**client, label: True})
