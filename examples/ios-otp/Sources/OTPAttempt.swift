import Foundation

/// Local entry-window policy. Authentication and server-side OTP expiry belong to your application.
struct OTPAttempt {
    enum State: Equatable { case entering, expired, cancelled, captured }
    private(set) var state: State = .entering
    private(set) var deadline: Date
    var code = ""

    init(now: Date) { deadline = now.addingTimeInterval(300) }

    var valid: Bool {
        code.utf8.count == 6 && code.utf8.allSatisfy { $0 >= 48 && $0 <= 57 }
    }

    mutating func expire(at now: Date) {
        if state == .entering && now >= deadline {
            code = ""
            state = .expired
        }
    }

    mutating func cancel() { code = ""; state = .cancelled }

    mutating func capture(at now: Date) -> Bool {
        expire(at: now)
        guard state == .entering && valid else { return false }
        code = ""
        state = .captured
        return true
    }
}
