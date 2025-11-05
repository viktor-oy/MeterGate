<?php

namespace App\Http\Middleware;

use App\Services\MeterGateClient;
use App\Services\OtelFile;
use Closure;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Symfony\Component\HttpFoundation\Response;

final readonly class CheckMeterGate
{
    public function __construct(
        private MeterGateClient $meterGate,
        private OtelFile $otel,
    ) {}

    /** @param Closure(Request): Response $next */
    public function handle(Request $request, Closure $next): Response
    {
        $requestId = $request->headers->get('X-Request-Id', (string) str()->uuid());

        if (config('metergate.mode') !== 'provider') {
            $response = $next($request);
            $response->headers->set('X-Request-Id', $requestId);
            return $response;
        }

        $decision = $this->meterGate->check($request);
        $this->otel->span('sample_api.provider_check', $requestId, ['decision' => $decision['reason'] ?? 'UNKNOWN']);

        if (($decision['allowed'] ?? false) !== true) {
            return new JsonResponse($decision, (int) ($decision['statusCode'] ?? 403), ['X-Request-Id' => $requestId]);
        }

        $response = $next($request);
        $response->headers->set('X-Request-Id', $requestId);
        return $response;
    }
}

