<?php

use App\Http\Middleware\CheckMeterGate;
use App\Services\TicketStore;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\Route;
use Nuwave\Lighthouse\Support\Http\Controllers\GraphQLController;

Route::get('/health', fn (): array => [
    'status' => 'ok',
    'service' => 'sample-api',
    'mode' => config('metergate.mode'),
]);

Route::get('/tickets', fn (TicketStore $store): array => $store->all());

Route::post('/graphql', GraphQLController::class)
    ->middleware(CheckMeterGate::class);

Route::post('/_demo/graphql-fallback', function (Request $request, TicketStore $store): JsonResponse {
    $query = (string) $request->input('query', '');
    $variables = (array) $request->input('variables', []);

    if (str_contains($query, 'addComment')) {
        return response()->json([
            'data' => [
                'addComment' => $store->addComment((string) $variables['ticketId'], (string) $variables['body']),
            ],
        ]);
    }

    if (str_contains($query, 'ticket(')) {
        return response()->json(['data' => ['ticket' => $store->find((string) $variables['id'])]]);
    }

    return response()->json(['data' => ['tickets' => $store->all()]]);
});

