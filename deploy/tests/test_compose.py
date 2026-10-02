"""Invariantes estáticos de la infraestructura local (no requieren Docker)."""

import re
from pathlib import Path

import pytest
import yaml

DEPLOY = Path(__file__).resolve().parents[1]
COMPOSE_TEXT = (DEPLOY / "docker-compose.yml").read_text(encoding="utf-8")
SERVICES = yaml.safe_load(COMPOSE_TEXT)["services"]
ENV_EXAMPLE = {
    k: v
    for k, _, v in (
        line.partition("=")
        for line in (DEPLOY / ".env.example").read_text(encoding="utf-8").splitlines()
        if line and not line.startswith("#")
    )
}
CORE = [n for n, s in SERVICES.items() if "core" in s["profiles"]]


def test_profiles():
    assert {p for s in SERVICES.values() for p in s["profiles"]} == {
        "core",
        "dev",
        "init",
    }
    assert {n for n, s in SERVICES.items() if "core" in s["profiles"]} == {
        "kafka",
        "redis-a",
        "redis-b",
        "timescaledb",
    }


@pytest.mark.parametrize("name", CORE)
def test_core_services_have_healthcheck(name):
    assert "healthcheck" in SERVICES[name]


@pytest.mark.parametrize("name", ["kafka-init", "kafka-ui"])
def test_dependents_wait_for_healthy_kafka(name):
    assert SERVICES[name]["depends_on"]["kafka"]["condition"] == "service_healthy"


def test_kafka_single_broker_settings():
    env = SERVICES["kafka"]["environment"]
    for key in (
        "KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR",
        "KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR",
        "KAFKA_TRANSACTION_STATE_LOG_MIN_ISR",
        "KAFKA_MIN_INSYNC_REPLICAS",
    ):
        assert int(env[key]) == 1, key
    assert env["KAFKA_PROCESS_ROLES"] == "broker,controller"
    assert str(env["KAFKA_AUTO_CREATE_TOPICS_ENABLE"]).lower() == "false"


def test_kafka_has_internal_and_external_listeners():
    env = SERVICES["kafka"]["environment"]
    advertised = env["KAFKA_ADVERTISED_LISTENERS"]
    assert "INTERNAL://kafka:9092" in advertised
    assert "EXTERNAL://localhost:" in advertised


def redis_conf(name: str) -> dict[str, str]:
    conf = {}
    for line in (
        (DEPLOY / "redis" / f"{name}.conf").read_text(encoding="utf-8").splitlines()
    ):
        key, _, value = line.partition(" ")
        if key and not key.startswith("#"):
            conf[key] = value
    return conf


def test_redis_a_evicts_by_ttl_without_persistence():
    conf = redis_conf("redis-a")
    assert conf["maxmemory-policy"] == "volatile-ttl"
    assert "maxmemory" in conf
    assert conf["appendonly"] == "no"


def test_redis_b_never_evicts_and_persists():
    conf = redis_conf("redis-b")
    assert conf["maxmemory-policy"] == "noeviction"
    assert conf["appendonly"] == "yes"
    assert "redis-b-data:/data" in SERVICES["redis-b"]["volumes"]


def test_timescaledb_has_persistent_volume_and_role_script():
    volumes = SERVICES["timescaledb"]["volumes"]
    assert any(v.startswith("timescaledb-data:") for v in volumes)
    assert any("01-roles.sh" in v for v in volumes)


def test_events_partitions_are_fixed_at_12():
    assert ENV_EXAMPLE["EVENTS_PARTITIONS"] == "12"


def test_every_compose_variable_is_defined_in_env_example():
    used = set(re.findall(r"\$\{(\w+)\}", COMPOSE_TEXT))
    assert used - set(ENV_EXAMPLE) == set()


def test_topics_script_is_idempotent_and_guards_events_partitions():
    script = (DEPLOY / "kafka" / "create-topics.sh").read_text(encoding="utf-8")
    assert "--if-not-exists" in script
    assert 'assert_partitions events "$EVENTS_PARTITIONS"' in script
    for topic in ("events", "alerts", "dlq", "actor-risk", "policies"):
        assert re.search(rf"^create {topic} ", script, re.MULTILINE), topic
