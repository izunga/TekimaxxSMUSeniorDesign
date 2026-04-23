use anyhow::{anyhow, Context, Result};
use base64::engine::general_purpose::STANDARD as BASE64;
use base64::Engine;
use hmac::{Hmac, Mac};
use serde::Serialize;
use sha2::Sha256;
use std::env;
use std::process::ExitCode;

type HmacSha256 = Hmac<Sha256>;

#[derive(Serialize)]
struct SignResponse {
    algorithm: &'static str,
    signature: String,
}

#[derive(Serialize)]
struct VerifyResponse {
    algorithm: &'static str,
    valid: bool,
}

fn main() -> ExitCode {
    match run() {
        Ok(()) => ExitCode::SUCCESS,
        Err(err) => {
            eprintln!("{err:#}");
            ExitCode::FAILURE
        }
    }
}

fn run() -> Result<()> {
    let mut args = env::args().skip(1);
    let command = args
        .next()
        .ok_or_else(|| anyhow!("usage: rust-crypto <sign|verify> --secret <secret> --message <message> [--signature <base64>]"))?;

    let mut secret = None;
    let mut message = None;
    let mut signature = None;

    while let Some(arg) = args.next() {
        match arg.as_str() {
            "--secret" => secret = args.next(),
            "--message" => message = args.next(),
            "--signature" => signature = args.next(),
            other => return Err(anyhow!("unknown argument: {other}")),
        }
    }

    let secret = secret.ok_or_else(|| anyhow!("missing --secret"))?;
    let message = message.ok_or_else(|| anyhow!("missing --message"))?;

    match command.as_str() {
        "sign" => {
            let sig = sign(&secret, &message)?;
            println!(
                "{}",
                serde_json::to_string_pretty(&SignResponse {
                    algorithm: "HMAC-SHA256",
                    signature: sig,
                })?
            );
        }
        "verify" => {
            let provided = signature.ok_or_else(|| anyhow!("missing --signature"))?;
            let valid = verify(&secret, &message, &provided)?;
            println!(
                "{}",
                serde_json::to_string_pretty(&VerifyResponse {
                    algorithm: "HMAC-SHA256",
                    valid,
                })?
            );
        }
        other => return Err(anyhow!("unknown command: {other}")),
    }

    Ok(())
}

fn sign(secret: &str, message: &str) -> Result<String> {
    let mut mac = HmacSha256::new_from_slice(secret.as_bytes())
        .context("invalid HMAC key length")?;
    mac.update(message.as_bytes());
    let bytes = mac.finalize().into_bytes();
    Ok(BASE64.encode(bytes))
}

fn verify(secret: &str, message: &str, provided: &str) -> Result<bool> {
    let provided = BASE64
        .decode(provided)
        .context("signature must be base64-encoded")?;
    let mut mac = HmacSha256::new_from_slice(secret.as_bytes())
        .context("invalid HMAC key length")?;
    mac.update(message.as_bytes());
    Ok(mac.verify_slice(&provided).is_ok())
}
