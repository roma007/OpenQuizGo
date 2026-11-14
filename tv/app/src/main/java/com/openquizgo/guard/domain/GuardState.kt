package com.openquizgo.guard.domain

enum class GuardState {
    UNBOUND,
    IDLE,
    WATCHING,
    QUIZ_REQUIRED,
    QUIZZING,
    GRANTED,
    BLOCKED,
}
