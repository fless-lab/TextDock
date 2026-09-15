package dev.textdock.otpsample;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.Arrays;
import java.util.Base64;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

/** Sample policy: exactly one standalone six-digit ASCII candidate, never verification. */
final class Otp {
    private static final Pattern CANDIDATE = Pattern.compile("(?<![A-Za-z0-9])[0-9]{6}(?![A-Za-z0-9])");

    static boolean valid(String code) { return code != null && code.matches("[0-9]{6}"); }

    static String extract(String message) {
        if (message == null || message.length() > 4096) return null;
        Matcher match = CANDIDATE.matcher(message);
        if (!match.find()) return null;
        String code = match.group();
        return match.find() ? null : code;
    }

    static String appHash(String packageName, String certificateHex) {
        try {
            byte[] digest = MessageDigest.getInstance("SHA-256").digest(
                (packageName + " " + certificateHex).getBytes(StandardCharsets.UTF_8));
            return Base64.getEncoder().withoutPadding().encodeToString(Arrays.copyOf(digest, 9)).substring(0, 11);
        } catch (NoSuchAlgorithmException impossible) {
            throw new IllegalStateException(impossible);
        }
    }

    /** Monotonic clock and generation prevent late callbacks from filling a cancelled attempt. */
    static final class Window {
        static final long DURATION = 300_000;
        private long generation;
        private long deadline;
        private boolean listening;

        long start(long now) { listening = true; deadline = now + DURATION; return ++generation; }
        boolean active(long token, long now) { return listening && token == generation && now < deadline; }
        void cancel() { listening = false; generation++; }
    }
}
