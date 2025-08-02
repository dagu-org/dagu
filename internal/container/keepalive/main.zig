const std = @import("std");

pub fn main() !void {
    // The OS will handle signal termination (SIGTERM, SIGINT) automatically
    while (true) {
        std.time.sleep(100 * std.time.ns_per_ms);
    }
}
