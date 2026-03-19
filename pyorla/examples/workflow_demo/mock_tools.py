"""Mock tool implementations for the workflow demo. Mirrors Go workflow_demo tools."""

from __future__ import annotations

import logging

from pyorla.tools import Tool, ToolResult, ToolSchema, tool_runner_from_schema

log = logging.getLogger(__name__)

_POLICIES: dict[str, str] = {
    "billing": """\
policy:
  billing:
    duplicate_charges:
      action: refund
      conditions:
        - verified_duplicate: true
        - within_days: 30
      sla: "Refund processed within 3 business days"
    subscription_cancellation:
      action: cancel_and_prorate
      conditions:
        - active_subscription: true
      sla: "Effective end of current billing cycle"\
""",
    "technical": """\
policy:
  technical:
    service_degradation:
      action: investigate_and_credit
      conditions:
        - confirmed_outage: true
        - duration_minutes: ">15"
      sla: "Resolution within 4 hours, credit if SLA missed"
    bug_report:
      action: escalate_to_engineering
      conditions:
        - reproducible: true
      sla: "Acknowledgment within 24 hours"\
""",
    "account": """\
policy:
  account:
    access_issues:
      action: reset_and_verify
      sla: "Resolved within 1 hour"
    data_request:
      action: export_data
      conditions:
        - identity_verified: true
      sla: "Data export within 48 hours"\
""",
    "shipping": """\
policy:
  shipping:
    lost_package:
      action: reship_or_refund
      conditions:
        - tracking_shows_lost: true
      sla: "Replacement shipped within 2 business days"\
""",
}


def read_policy_yaml_tool() -> Tool:
    """read_policy_yaml — simulates reading company support policy."""

    def _run(input_args: ToolSchema) -> ToolSchema:
        category = input_args.get("category", "")
        policy = _POLICIES.get(category, f"No specific policy found for category: {category}.")
        return {"policy_document": policy}

    return Tool(
        name="read_policy_yaml",
        description=(
            "Read the company support policy document for a given category. "
            "Returns the policy rules as structured text."
        ),
        input_schema={
            "type": "object",
            "properties": {
                "category": {
                    "type": "string",
                    "description": "The ticket category to look up policy for",
                },
            },
            "required": ["category"],
        },
        run=tool_runner_from_schema(_run),
    )


def send_email_tool() -> Tool:
    """send_email — simulates sending an email."""

    def _run(input_args: ToolSchema) -> ToolSchema:
        to = input_args.get("to", "unknown")
        subject = input_args.get("subject", "")
        log.info("[send_email] To: %s | Subject: %s", to, subject)
        return {"status": "sent", "message_id": f"msg-{to}-001"}

    return Tool(
        name="send_email",
        description="Send an email to a recipient with the given subject and body.",
        input_schema={
            "type": "object",
            "properties": {
                "to": {"type": "string", "description": "Recipient email address"},
                "subject": {"type": "string", "description": "Email subject line"},
                "body": {"type": "string", "description": "Email body text"},
            },
            "required": ["to", "subject", "body"],
        },
        run=tool_runner_from_schema(_run),
    )


def read_team_descriptions_tool() -> Tool:
    """read_team_descriptions — simulates reading internal team info."""

    def _run(_input: ToolSchema) -> ToolSchema:
        return {
            "teams": [
                {
                    "name": "billing_ops",
                    "description": "Handles refunds, subscription changes, payment disputes, and invoice corrections.",
                    "email": "billing-ops@company.com",
                },
                {
                    "name": "technical_support",
                    "description": "Handles service outages, performance issues, bug reports, and API problems.",
                    "email": "tech-support@company.com",
                },
                {
                    "name": "account_management",
                    "description": "Handles account access, data requests, plan upgrades, and enterprise onboarding.",
                    "email": "account-mgmt@company.com",
                },
                {
                    "name": "escalation_team",
                    "description": "Handles critical/VIP issues, multi-department problems, and unresolved complaints.",
                    "email": "escalation@company.com",
                },
            ]
        }

    return Tool(
        name="read_team_descriptions",
        description="Read descriptions of internal support teams to determine the best routing destination.",
        input_schema={"type": "object", "properties": {}},
        run=tool_runner_from_schema(_run),
    )


def send_ticket_tool() -> Tool:
    """send_ticket — simulates creating an internal support ticket."""

    def _run(input_args: ToolSchema) -> ToolSchema:
        team = input_args.get("team", "unknown")
        priority = input_args.get("priority", "medium")
        log.info("[send_ticket] Team: %s | Priority: %s", team, priority)
        return {"ticket_id": f"TKT-{team}-42", "status": "created"}

    return Tool(
        name="send_ticket",
        description="Create and send an internal support ticket to the designated team.",
        input_schema={
            "type": "object",
            "properties": {
                "team": {"type": "string", "description": "The team to route the ticket to"},
                "priority": {
                    "type": "string",
                    "enum": ["critical", "high", "medium", "low"],
                    "description": "Ticket priority level",
                },
                "summary": {"type": "string", "description": "Brief summary of the issue"},
                "customer_email": {"type": "string", "description": "Customer email for follow-up"},
            },
            "required": ["team", "priority", "summary"],
        },
        run=tool_runner_from_schema(_run),
    )
