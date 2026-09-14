package dev.textdock.gateway;

/** Serializes short preference updates, never network or telephony calls. */
final class GatewayState {
    static final Object LOCK = new Object();
    private GatewayState() {}
}
