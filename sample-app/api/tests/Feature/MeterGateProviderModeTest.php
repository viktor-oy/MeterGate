<?php

namespace Tests\Feature;

use Illuminate\Support\Facades\Http;
use Tests\TestCase;

final class MeterGateProviderModeTest extends TestCase
{
    public function test_provider_mode_blocks_denied_graphql_requests(): void
    {
        Http::fake([
            '*' => Http::response([
                'allowed' => false,
                'statusCode' => 429,
                'reason' => 'RATE_LIMIT_EXCEEDED',
            ], 200),
        ]);

        $response = $this->postJson('/graphql', [
            'query' => 'query { tickets { id subject status } }',
        ], ['x-api-key' => 'mg_demo_local_plaintext_key']);

        $response->assertStatus(429);
        $response->assertJson(['reason' => 'RATE_LIMIT_EXCEEDED']);
    }
}
