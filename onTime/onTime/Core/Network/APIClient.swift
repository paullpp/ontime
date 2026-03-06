import Foundation

actor APIClient {
    static let shared = APIClient()

    // Change to your Mac's IP when testing on a real device.
    private let baseURL = URL(string: "http://localhost:8080")!
    private let session = URLSession.shared
    private let tokenStore = TokenStore.shared

    // MARK: - Auth

    func signInWithApple(identityToken: String, deviceToken: String? = nil) async throws -> AuthResponse {
        var body: [String: Any] = ["identity_token": identityToken]
        if let deviceToken { body["device_token"] = deviceToken }
        return try await post("/api/v1/auth/apple", json: body, authenticated: false)
    }

    func refreshTokens() async throws -> AuthResponse {
        guard let token = await tokenStore.refreshToken else { throw APIError.unauthorized }
        return try await post("/api/v1/auth/refresh", json: ["refresh_token": token], authenticated: false)
    }

    func logout() async throws {
        try await perform(method: "DELETE", path: "/api/v1/auth/logout")
    }

    // MARK: - Devices

    @discardableResult
    func registerDevice(apnsToken: String) async throws -> Device {
        return try await post("/api/v1/devices", json: ["apns_token": apnsToken])
    }

    func deregisterDevice(id: UUID) async throws {
        try await perform(method: "DELETE", path: "/api/v1/devices/\(id)")
    }

    // MARK: - Trips

    func listTrips() async throws -> [Trip] {
        return try await get("/api/v1/trips")
    }

    func createTrip(_ req: CreateTripRequest) async throws -> Trip {
        return try await post("/api/v1/trips", encodable: req)
    }

    func getTrip(id: UUID) async throws -> Trip {
        return try await get("/api/v1/trips/\(id)")
    }

    func updateTrip(id: UUID, _ req: UpdateTripRequest) async throws -> Trip {
        return try await put("/api/v1/trips/\(id)", encodable: req)
    }

    func cancelTrip(id: UUID) async throws {
        try await perform(method: "DELETE", path: "/api/v1/trips/\(id)")
    }

    func activateTrip(id: UUID) async throws {
        try await perform(method: "POST", path: "/api/v1/trips/\(id)/activate")
    }

    // MARK: - Private helpers

    private func get<T: Decodable>(_ path: String) async throws -> T {
        let req = try await buildRequest(method: "GET", path: path, body: nil, authenticated: true)
        return try await execute(req)
    }

    private func post<T: Decodable>(_ path: String, json: [String: Any], authenticated: Bool = true) async throws -> T {
        let body = try JSONSerialization.data(withJSONObject: json)
        let req = try await buildRequest(method: "POST", path: path, body: body, authenticated: authenticated)
        return try await execute(req)
    }

    private func post<T: Decodable, B: Encodable>(_ path: String, encodable: B) async throws -> T {
        let body = try encoder.encode(encodable)
        let req = try await buildRequest(method: "POST", path: path, body: body, authenticated: true)
        return try await execute(req)
    }

    private func put<T: Decodable, B: Encodable>(_ path: String, encodable: B) async throws -> T {
        let body = try encoder.encode(encodable)
        let req = try await buildRequest(method: "PUT", path: path, body: body, authenticated: true)
        return try await execute(req)
    }

    private func perform(method: String, path: String) async throws {
        let req = try await buildRequest(method: method, path: path, body: nil, authenticated: true)
        let (_, response) = try await session.data(for: req)
        let http = response as! HTTPURLResponse
        guard (200..<300).contains(http.statusCode) else {
            if http.statusCode == 401 { throw APIError.unauthorized }
            if http.statusCode == 404 { throw APIError.notFound }
            throw APIError.serverError(http.statusCode, nil)
        }
    }

    private func buildRequest(method: String, path: String, body: Data?, authenticated: Bool) async throws -> URLRequest {
        var req = URLRequest(url: baseURL.appending(path: path))
        req.httpMethod = method
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        req.httpBody = body
        if authenticated {
            guard let token = await tokenStore.accessToken else { throw APIError.unauthorized }
            req.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }
        return req
    }

    private func execute<T: Decodable>(_ request: URLRequest, retrying: Bool = true) async throws -> T {
        let (data, response) = try await session.data(for: request)
        let http = response as! HTTPURLResponse

        if http.statusCode == 401 && retrying {
            let tokens = try await refreshTokens()
            await tokenStore.save(accessToken: tokens.accessToken, refreshToken: tokens.refreshToken)
            var retried = request
            retried.setValue("Bearer \(tokens.accessToken)", forHTTPHeaderField: "Authorization")
            return try await execute(retried, retrying: false)
        }

        guard (200..<300).contains(http.statusCode) else {
            let msg = (try? JSONDecoder().decode([String: String].self, from: data))?["error"]
            if http.statusCode == 401 { throw APIError.unauthorized }
            if http.statusCode == 404 { throw APIError.notFound }
            throw APIError.serverError(http.statusCode, msg)
        }

        do {
            return try decoder.decode(T.self, from: data)
        } catch {
            throw APIError.decodingError(error)
        }
    }

    // MARK: - Coding

    private let encoder: JSONEncoder = {
        let e = JSONEncoder()
        e.dateEncodingStrategy = .iso8601
        return e
    }()

    private let decoder: JSONDecoder = makeDecoder()
}

func makeDecoder() -> JSONDecoder {
    let d = JSONDecoder()
    let full = ISO8601DateFormatter()
    full.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
    let basic = ISO8601DateFormatter()
    basic.formatOptions = [.withInternetDateTime]
    d.dateDecodingStrategy = .custom { decoder in
        let c = try decoder.singleValueContainer()
        let s = try c.decode(String.self)
        if let date = full.date(from: s) { return date }
        if let date = basic.date(from: s) { return date }
        throw DecodingError.dataCorruptedError(in: c, debugDescription: "Cannot parse date: \(s)")
    }
    return d
}
