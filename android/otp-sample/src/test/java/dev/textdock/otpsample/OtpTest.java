package dev.textdock.otpsample;

import org.junit.Test;
import static org.junit.Assert.*;

public class OtpTest {
    @Test public void signingHashMatchesIndependentSha256Vector() {
        assertEquals("1LOI4WphBgH", Otp.appHash("dev.textdock.otpsample", "0123456789abcdef"));
        assertNotEquals(Otp.appHash("dev.textdock.otpsample", "0123456789abcdef"),
            Otp.appHash("dev.textdock.otpsample", "0123456789abcdee"));
    }

    @Test public void onlyOneStandaloneAsciiCodeIsAccepted() {
        assertEquals("482193", Otp.extract("<#> Your verification code is 482193\nFA+9qCX9VSu"));
        assertNull(Otp.extract("482193 or 928471"));
        assertNull(Otp.extract("order 14821930"));
        assertNull(Otp.extract("token A482193B"));
        assertNull(Otp.extract("４８２１９３"));
        assertNull(Otp.extract(null));
        assertTrue(Otp.valid("000001"));
        assertFalse(Otp.valid("1234567"));
        assertFalse(Otp.valid(" 123456"));
    }

    @Test public void lateResultsCannotReviveCancelledOrExpiredWindows() {
        Otp.Window window = new Otp.Window();
        long first = window.start(100);
        assertTrue(window.active(first, 101));
        assertFalse(window.active(first, 100 + Otp.Window.DURATION));
        window.cancel();
        assertFalse(window.active(first, 101));
        long next = window.start(200);
        assertFalse(window.active(first, 201));
        assertTrue(window.active(next, 201));
    }
}
