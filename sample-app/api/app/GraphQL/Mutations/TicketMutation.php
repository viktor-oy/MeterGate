<?php

namespace App\GraphQL\Mutations;

use App\Services\TicketStore;

final readonly class TicketMutation
{
    public function __construct(private TicketStore $tickets) {}

    /** @param array{ticketId: string, body: string} $args */
    public function addComment(mixed $_root, array $args): array
    {
        return $this->tickets->addComment($args['ticketId'], $args['body']);
    }
}

