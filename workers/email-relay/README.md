# Verification email relay

This Worker is the only email transport used by the agent shop for registration
and password-reset codes. It sends through Cloudflare Email Service with the
fixed sender `老实人AI VIP <no-reply@laoshirenvip.com>`.

- Worker secret: `RELAY_TOKEN`
- Agent Switch source secret: `LAOSHIRENVIP_AGENT_SHOP_EMAIL_RELAY_SECRET`
- Agent runtime variables: `EMAIL_VERIFICATION_RELAY_URL` and
  `EMAIL_VERIFICATION_RELAY_TOKEN`

Never commit the token or copy it into a project `.env` file.
