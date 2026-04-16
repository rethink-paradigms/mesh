import pytest
import pulumi

from mesh.infrastructure.provision_node.security_groups import (
    INTERNAL_CIDR,
    PUBLIC_CIDR,
    REQUIRED_TCP_PORTS,
    SENSITIVE_PORTS,
    TAILSCALE_PORT,
    IngressRule,
    create_mesh_security_group,
    get_expanded_tcp_ports,
    get_mesh_egress_rule,
    get_mesh_ingress_rules,
    get_security_group_tags,
    validate_no_public_access_on_sensitive_ports,
)

created_resources: dict = {}


class MyMocks(pulumi.runtime.Mocks):
    def new_resource(self, args: pulumi.runtime.MockResourceArgs):
        created_resources[args.name] = args.inputs
        state = args.inputs.copy()
        return [args.name + "_id", state]

    def call(self, args: pulumi.runtime.MockCallArgs):
        return {}


@pytest.fixture(autouse=True)
def setup_mocks():
    created_resources.clear()
    pulumi.runtime.set_mocks(MyMocks(), preview=False)


def _get_port(rule, key_camel, key_snake):
    val = rule.get(key_camel) or rule.get(key_snake) or 0
    return int(val)


def _get_protocol(rule):
    return rule.get("protocol", "")


def _get_cidr_blocks(rule):
    return rule.get("cidrBlocks") or rule.get("cidr_blocks") or []


def _extract_ports(ingress_list):
    tcp_ports: set = set()
    udp_ports: set = set()
    for rule in ingress_list:
        from_port = _get_port(rule, "fromPort", "from_port")
        to_port = _get_port(rule, "toPort", "to_port")
        protocol = _get_protocol(rule)
        for p in range(from_port, to_port + 1):
            if protocol == "tcp":
                tcp_ports.add(p)
            elif protocol == "udp":
                udp_ports.add(p)
    return tcp_ports, udp_ports


class TestRequiredInboundPorts:
    def test_ssh_port_not_allowed(self):
        ports = get_expanded_tcp_ports(get_mesh_ingress_rules())
        assert 22 not in ports

    def test_http_port_allowed(self):
        ports = get_expanded_tcp_ports(get_mesh_ingress_rules())
        assert 80 in ports

    def test_https_port_allowed(self):
        ports = get_expanded_tcp_ports(get_mesh_ingress_rules())
        assert 443 in ports

    def test_nomad_http_port_allowed(self):
        ports = get_expanded_tcp_ports(get_mesh_ingress_rules())
        assert 4646 in ports

    def test_consul_http_port_allowed(self):
        ports = get_expanded_tcp_ports(get_mesh_ingress_rules())
        assert 8500 in ports

    def test_consul_rpc_port_range_allowed(self):
        ports = get_expanded_tcp_ports(get_mesh_ingress_rules())
        assert 8300 in ports
        assert 8301 in ports
        assert 8302 in ports

    def test_all_required_tcp_ports_present(self):
        ports = get_expanded_tcp_ports(get_mesh_ingress_rules())
        missing = REQUIRED_TCP_PORTS - ports
        assert not missing, f"Missing required TCP ports: {missing}"

    @pulumi.runtime.test
    def test_pulumi_security_group_has_ingress_rules(self):
        sg = create_mesh_security_group("test-ingress", "server")

        def check(sg_id):
            assert "test-ingress" in created_resources
            inputs = created_resources["test-ingress"]
            ingress = inputs.get("ingress", [])
            tcp_ports, _ = _extract_ports(ingress)
            assert 22 not in tcp_ports
            for port in [80, 443, 4646, 8500, 8300, 8301, 8302]:
                assert port in tcp_ports, (
                    f"Port {port} missing from security group ingress"
                )

        return sg.id.apply(check)


class TestOutboundTraffic:
    def test_all_outbound_allowed_by_data(self):
        egress = get_mesh_egress_rule()
        assert egress.protocol == "-1"
        assert egress.from_port == 0
        assert egress.to_port == 0
        assert PUBLIC_CIDR in egress.cidr_blocks

    def test_egress_rule_exists(self):
        egress = get_mesh_egress_rule()
        assert isinstance(egress, IngressRule)

    @pulumi.runtime.test
    def test_pulumi_security_group_has_open_egress(self):
        sg = create_mesh_security_group("test-egress", "server")

        def check(sg_id):
            assert "test-egress" in created_resources
            inputs = created_resources["test-egress"]
            egress = inputs.get("egress", [])
            assert len(egress) >= 1
            rule = egress[0]
            protocol = _get_protocol(rule)
            from_port = _get_port(rule, "fromPort", "from_port")
            to_port = _get_port(rule, "toPort", "to_port")
            cidrs = _get_cidr_blocks(rule)
            assert protocol == "-1"
            assert from_port == 0
            assert to_port == 0
            assert PUBLIC_CIDR in cidrs

        return sg.id.apply(check)


class TestTailscalePort:
    def test_tailscale_port_in_ingress_rules(self):
        rules = get_mesh_ingress_rules()
        tailscale = [r for r in rules if r.from_port == TAILSCALE_PORT]
        assert len(tailscale) == 1

    def test_tailscale_protocol_is_udp(self):
        rules = get_mesh_ingress_rules()
        tailscale = [r for r in rules if r.from_port == TAILSCALE_PORT][0]
        assert tailscale.protocol == "udp"

    def test_tailscale_allows_public_access(self):
        rules = get_mesh_ingress_rules()
        tailscale = [r for r in rules if r.from_port == TAILSCALE_PORT][0]
        assert PUBLIC_CIDR in tailscale.cidr_blocks

    @pulumi.runtime.test
    def test_pulumi_security_group_includes_tailscale(self):
        sg = create_mesh_security_group("test-tailscale", "server")

        def check(sg_id):
            assert "test-tailscale" in created_resources
            inputs = created_resources["test-tailscale"]
            ingress = inputs.get("ingress", [])
            _, udp_ports = _extract_ports(ingress)
            assert TAILSCALE_PORT in udp_ports

        return sg.id.apply(check)


class TestSecurityGroupTags:
    def test_project_tag(self):
        tags = get_security_group_tags("node-1", "server")
        assert tags["Project"] == "distributed-mesh-platform"

    def test_role_tag_server(self):
        tags = get_security_group_tags("node-1", "server")
        assert tags["Role"] == "server"

    def test_role_tag_client(self):
        tags = get_security_group_tags("node-2", "client")
        assert tags["Role"] == "client"

    def test_name_tag(self):
        tags = get_security_group_tags("my-node", "server")
        assert tags["Name"] == "my-node"

    def test_extra_tags_merged(self):
        tags = get_security_group_tags(
            "node-1", "server", {"Environment": "production"}
        )
        assert tags["Environment"] == "production"
        assert tags["Role"] == "server"
        assert tags["Project"] == "distributed-mesh-platform"

    @pulumi.runtime.test
    def test_pulumi_security_group_tags(self):
        sg = create_mesh_security_group(
            "test-tags", "client", extra_tags={"Environment": "staging"}
        )

        def check(sg_id):
            assert "test-tags" in created_resources
            inputs = created_resources["test-tags"]
            tags = inputs.get("tags", {})
            assert tags.get("Project") == "distributed-mesh-platform"
            assert tags.get("Role") == "client"
            assert tags.get("Name") == "test-tags"
            assert tags.get("Environment") == "staging"

        return sg.id.apply(check)


class TestNoOverlyPermissiveRules:
    def test_sensitive_ports_not_publicly_accessible(self):
        rules = get_mesh_ingress_rules()
        assert validate_no_public_access_on_sensitive_ports(rules) is True

    def test_consul_rpc_uses_internal_cidr(self):
        rules = get_mesh_ingress_rules()
        consul_rules = [r for r in rules if r.from_port == 8300]
        assert len(consul_rules) == 1
        assert INTERNAL_CIDR in consul_rules[0].cidr_blocks
        assert PUBLIC_CIDR not in consul_rules[0].cidr_blocks

    def test_consul_serf_lan_uses_internal_cidr(self):
        rules = get_mesh_ingress_rules()
        serf_rules = [r for r in rules if r.from_port == 8300 and r.to_port == 8302]
        assert len(serf_rules) == 1
        assert INTERNAL_CIDR in serf_rules[0].cidr_blocks
        assert PUBLIC_CIDR not in serf_rules[0].cidr_blocks

    def test_ssh_is_not_publicly_accessible(self):
        rules = get_mesh_ingress_rules()
        ssh_rules = [r for r in rules if r.from_port == 22]
        assert len(ssh_rules) == 0

    def test_nomad_http_uses_internal_cidr(self):
        rules = get_mesh_ingress_rules()
        nomad_rules = [r for r in rules if r.from_port == 4646]
        assert len(nomad_rules) == 1
        assert INTERNAL_CIDR in nomad_rules[0].cidr_blocks
        assert PUBLIC_CIDR not in nomad_rules[0].cidr_blocks

    def test_consul_http_uses_internal_cidr(self):
        rules = get_mesh_ingress_rules()
        consul_http_rules = [r for r in rules if r.from_port == 8500]
        assert len(consul_http_rules) == 1
        assert INTERNAL_CIDR in consul_http_rules[0].cidr_blocks
        assert PUBLIC_CIDR not in consul_http_rules[0].cidr_blocks

    def test_validation_catches_public_sensitive_port(self):
        malicious_rules = [
            IngressRule("tcp", 4646, 4646, [PUBLIC_CIDR], "Bad rule"),
        ]
        assert validate_no_public_access_on_sensitive_ports(malicious_rules) is False

    @pulumi.runtime.test
    def test_pulumi_security_group_no_public_sensitive_ports(self):
        sg = create_mesh_security_group("test-permissive", "server")

        def check(sg_id):
            assert "test-permissive" in created_resources
            inputs = created_resources["test-permissive"]
            ingress = inputs.get("ingress", [])
            for rule in ingress:
                from_port = _get_port(rule, "fromPort", "from_port")
                to_port = _get_port(rule, "toPort", "to_port")
                cidrs = _get_cidr_blocks(rule)
                for p in range(from_port, to_port + 1):
                    if p in SENSITIVE_PORTS:
                        assert PUBLIC_CIDR not in cidrs, (
                            f"Port {p} should not be publicly accessible"
                        )

        return sg.id.apply(check)
