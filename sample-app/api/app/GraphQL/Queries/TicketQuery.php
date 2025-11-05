<?php

namespace App\GraphQL\Queries;

use App\Services\TicketStore;

final readonly class TicketQuery
{
    public function __construct(private TicketStore $tickets) {}

    /** @return array<int, array<string, mixed>> */
    public function list(): array
    {
        return $this->tickets->all();
    }

    /** @param array{id: string} $args */
    public function find(mixed $_root, array $args): ?array
    {
        return $this->tickets->find($args['id']);
    }
}

