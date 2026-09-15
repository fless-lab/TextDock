import XCTest
@testable import TextDockOTP

final class OTPAttemptTests: XCTestCase {
    private let now = Date(timeIntervalSince1970: 100)

    func testExactAsciiPolicyDoesNotTruncateOrAcceptUnicodeDigits() {
        var attempt = OTPAttempt(now: now)
        for value in ["12345", "1234567", "１２３４５６", " 123456", "12345a"] {
            attempt.code = value
            XCTAssertFalse(attempt.capture(at: now))
        }
        attempt.code = "000001"
        XCTAssertTrue(attempt.capture(at: now))
        XCTAssertEqual(attempt.state, .captured)
        XCTAssertEqual(attempt.code, "")
    }

    func testExpiredAndCancelledAttemptsCannotAcceptLateAutofill() {
        var attempt = OTPAttempt(now: now)
        attempt.code = "482193"
        XCTAssertFalse(attempt.capture(at: now.addingTimeInterval(300)))
        XCTAssertEqual(attempt.state, .expired)
        XCTAssertEqual(attempt.code, "")
        attempt = OTPAttempt(now: now)
        attempt.cancel()
        attempt.code = "482193"
        XCTAssertFalse(attempt.capture(at: now))
        XCTAssertEqual(attempt.state, .cancelled)
    }

    func testReturningAfterDeadlineClearsCode() {
        var attempt = OTPAttempt(now: now)
        attempt.code = "482193"
        attempt.expire(at: now.addingTimeInterval(301))
        XCTAssertEqual(attempt.state, .expired)
        XCTAssertEqual(attempt.code, "")
    }
}
