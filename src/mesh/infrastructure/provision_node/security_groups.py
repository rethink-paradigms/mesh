from dataclasses import dataclass
from typing import Dict, List, Optional

import pulumi_aws as aws

INTERNAL_CIDR = "10.0.0.0/8"
PUBLIC_CIDR = "0.0.0.0/0"
TAILSCALE_PORT = 41641
SENSITIVE_PORTS = frozenset({4646, 8300, 8301, 8302, 8500})
REQUIRED_TCP_PORTS = frozenset({80, 443, 4646, 8500, 8300, 8301, 8302})


@dataclass
class IngressRule:
    protocol: str
    from_port: int
    to_port: int
    cidr_blocks: List[str]
    description: str = ""


MESH_INGRESS_RULES: List[IngressRule] = [
    IngressRule("tcp", 80, 80, [PUBLIC_CIDR], "HTTP"),
    IngressRule("tcp", 443, 443, [PUBLIC_CIDR], "HTTPS"),
    IngressRule("tcp", 4646, 4646, [INTERNAL_CIDR], "Nomad HTTP API"),
    IngressRule("tcp", 8500, 8500, [INTERNAL_CIDR], "Consul HTTP API"),
    IngressRule("tcp", 8300, 8302, [INTERNAL_CIDR], "Consul RPC and Serf"),
    IngressRule("udp", 41641, 41641, [PUBLIC_CIDR], "Tailscale mesh VPN"),
]

MESH_EGRESS_RULE = IngressRule("-1", 0, 0, [PUBLIC_CIDR], "Allow all outbound traffic")


def get_mesh_ingress_rules() -> List[IngressRule]:
    return list(MESH_INGRESS_RULES)


def get_mesh_egress_rule() -> IngressRule:
    return MESH_EGRESS_RULE


def get_security_group_tags(
    name: str, role: str, extra: Optional[Dict[str, str]] = None
) -> Dict[str, str]:
    tags: Dict[str, str] = {
        "Name": name,
        "Project": "distributed-mesh-platform",
        "Role": role,
    }
    if extra:
        tags.update(extra)
    return tags


def validate_no_public_access_on_sensitive_ports(rules: List[IngressRule]) -> bool:
    for rule in rules:
        for port in range(rule.from_port, rule.to_port + 1):
            if port in SENSITIVE_PORTS and PUBLIC_CIDR in rule.cidr_blocks:
                return False
    return True


def get_expanded_tcp_ports(rules: List[IngressRule]) -> set:
    ports: set = set()
    for rule in rules:
        if rule.protocol == "tcp":
            for port in range(rule.from_port, rule.to_port + 1):
                ports.add(port)
    return ports


def create_mesh_security_group(
    name: str,
    role: str,
    vpc_id: Optional[str] = None,
    extra_tags: Optional[Dict[str, str]] = None,
) -> aws.ec2.SecurityGroup:
    tags = get_security_group_tags(name, role, extra_tags)

    ingress = [
        aws.ec2.SecurityGroupIngressArgs(
            protocol=r.protocol,
            from_port=r.from_port,
            to_port=r.to_port,
            cidr_blocks=r.cidr_blocks,
            description=r.description,
        )
        for r in MESH_INGRESS_RULES
    ]

    egress = [
        aws.ec2.SecurityGroupEgressArgs(
            protocol=MESH_EGRESS_RULE.protocol,
            from_port=MESH_EGRESS_RULE.from_port,
            to_port=MESH_EGRESS_RULE.to_port,
            cidr_blocks=MESH_EGRESS_RULE.cidr_blocks,
            description=MESH_EGRESS_RULE.description,
        )
    ]

    return aws.ec2.SecurityGroup(
        name,
        description=f"Mesh platform {role} node security group",
        vpc_id=vpc_id,
        ingress=ingress,
        egress=egress,
        tags=tags,
    )
