<?php

namespace App\Services;

use Illuminate\Http\Client\ConnectionException;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\Http;

final class MeterGateClient
{
    /** @return array<string, mixed> */
    public function check(Request $request): array
    {
        $apiKeyHeader = config('metergate.api_key_header');
        $apiKey = (string) $request->headers->get($apiKeyHeader, '');

        try {
            $response = Http::timeout(2)
                ->withHeaders(['X-Request-Id' => $request->headers->get('X-Request-Id', (string) str()->uuid())])
                ->post(config('metergate.provider_url'), [
                    'method' => $request->method(),
                    'path' => '/'.$request->path(),
                    'apiKey' => $apiKey !== '' ? $apiKey : null,
                ]);
        } catch (ConnectionException) {
            return [
                'allowed' => false,
                'statusCode' => 503,
                'reason' => 'METERGATE_UNAVAILABLE',
            ];
        }

        return $response->json();
    }
}

