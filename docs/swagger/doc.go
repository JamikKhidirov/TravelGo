package swagger

import (
	"github.com/swaggo/swag"
)

// swaggerSpec contains the OpenAPI 3.0 specification for GoTravel API.
const swaggerSpec = `{
  "openapi": "3.0.3",
  "info": {
    "title": "GoTravel API",
    "description": "Travel microservice written in Go. Collects data from MSP servers and other public sources (no API keys required), stores it in PostgreSQL, caches in Redis, streams events via Kafka, and exposes REST + WebSocket endpoints for the client.",
    "version": "1.0.0",
    "contact": {
      "name": "GoTravel Team",
      "email": "support@gotravel.local"
    }
  },
  "servers": [
    { "url": "http://localhost:8080", "description": "Local development server" },
    { "url": "http://travel-app:8080", "description": "Docker service" }
  ],
  "tags": [
    { "name": "health", "description": "Service health check" },
    { "name": "destinations", "description": "Travel destinations" },
    { "name": "tours", "description": "Tours and packages" },
    { "name": "events", "description": "Change feed / event log" },
    { "name": "sync", "description": "Data synchronization from MSP sources" },
    { "name": "realtime", "description": "WebSocket real-time updates" }
  ],
  "paths": {
    "/health": {
      "get": {
        "summary": "Health check",
        "tags": ["health"],
        "responses": {
          "200": {
            "description": "Service is healthy",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/Response" }
              }
            }
          }
        }
      }
    },
    "/api/v1/destinations": {
      "get": {
        "summary": "List destinations",
        "tags": ["destinations"],
        "parameters": [
          { "in": "query", "name": "country", "schema": { "type": "string" }, "description": "Filter by country" },
          { "in": "query", "name": "city", "schema": { "type": "string" }, "description": "Filter by city" },
          { "in": "query", "name": "page", "schema": { "type": "integer", "minimum": 1, "default": 1 } },
          { "in": "query", "name": "page_size", "schema": { "type": "integer", "minimum": 1, "maximum": 100, "default": 20 } }
        ],
        "responses": {
          "200": {
            "description": "List of destinations",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "success": { "type": "boolean" },
                    "data": { "type": "array", "items": { "$ref": "#/components/schemas/Destination" } },
                    "meta": { "$ref": "#/components/schemas/PaginationMeta" }
                  }
                }
              }
            }
          }
        }
      },
      "post": {
        "summary": "Create a destination",
        "tags": ["destinations"],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": { "$ref": "#/components/schemas/Destination" }
            }
          }
        },
        "responses": {
          "201": {
            "description": "Destination created",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/Response" }
              }
            }
          }
        }
      }
    },
    "/api/v1/destinations/{id}": {
      "get": {
        "summary": "Get destination by ID",
        "tags": ["destinations"],
        "parameters": [
          { "in": "path", "name": "id", "required": true, "schema": { "type": "string", "format": "uuid" } }
        ],
        "responses": {
          "200": { "description": "Destination found", "content": { "application/json": { "schema": { "$ref": "#/components/schemas/Response" } } } },
          "404": { "description": "Not found" }
        }
      }
    },
    "/api/v1/tours": {
      "get": {
        "summary": "List tours",
        "tags": ["tours"],
        "parameters": [
          { "in": "query", "name": "destination_id", "schema": { "type": "string", "format": "uuid" } },
          { "in": "query", "name": "available", "schema": { "type": "boolean" } },
          { "in": "query", "name": "page", "schema": { "type": "integer", "default": 1 } },
          { "in": "query", "name": "page_size", "schema": { "type": "integer", "default": 20 } }
        ],
        "responses": {
          "200": {
            "description": "List of tours",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "success": { "type": "boolean" },
                    "data": { "type": "array", "items": { "$ref": "#/components/schemas/Tour" } },
                    "meta": { "$ref": "#/components/schemas/PaginationMeta" }
                  }
                }
              }
            }
          }
        }
      },
      "post": {
        "summary": "Create a tour",
        "tags": ["tours"],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": { "$ref": "#/components/schemas/Tour" }
            }
          }
        },
        "responses": {
          "201": {
            "description": "Tour created",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/Response" }
              }
            }
          }
        }
      }
    },
    "/api/v1/tours/{id}": {
      "get": {
        "summary": "Get tour by ID",
        "tags": ["tours"],
        "parameters": [
          { "in": "path", "name": "id", "required": true, "schema": { "type": "string", "format": "uuid" } }
        ],
        "responses": {
          "200": { "description": "Tour found", "content": { "application/json": { "schema": { "$ref": "#/components/schemas/Response" } } } }
        }
      }
    },
    "/api/v1/events": {
      "get": {
        "summary": "List travel events (change feed)",
        "tags": ["events"],
        "parameters": [
          { "in": "query", "name": "aggregate", "schema": { "type": "string" } },
          { "in": "query", "name": "aggregate_id", "schema": { "type": "string", "format": "uuid" } },
          { "in": "query", "name": "limit", "schema": { "type": "integer", "default": 50 } }
        ],
        "responses": {
          "200": {
            "description": "List of events",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "success": { "type": "boolean" },
                    "data": { "type": "array", "items": { "$ref": "#/components/schemas/TravelEvent" } }
                  }
                }
              }
            }
          }
        }
      }
    },
    "/api/v1/sync": {
      "post": {
        "summary": "Sync data from MSP sources",
        "tags": ["sync"],
        "responses": {
          "200": {
            "description": "Sync started",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/Response" }
              }
            }
          }
        }
      }
    },
    "/ws": {
      "get": {
        "summary": "WebSocket endpoint (generic)",
        "tags": ["realtime"],
        "responses": {
          "101": { "description": "Switching Protocols" }
        }
      }
    },
    "/ws/events": {
      "get": {
        "summary": "WebSocket endpoint for real-time travel events",
        "tags": ["realtime"],
        "responses": {
          "101": { "description": "Switching Protocols" }
        }
      }
    }
  },
  "components": {
    "schemas": {
      "Response": {
        "type": "object",
        "properties": {
          "success": { "type": "boolean" },
          "data": { "type": "object", "nullable": true },
          "error": { "type": "string", "nullable": true },
          "meta": { "type": "object", "nullable": true }
        }
      },
      "Destination": {
        "type": "object",
        "required": ["name", "country", "city"],
        "properties": {
          "id": { "type": "string", "format": "uuid" },
          "name": { "type": "string" },
          "country": { "type": "string" },
          "city": { "type": "string" },
          "description": { "type": "string" },
          "image_url": { "type": "string" },
          "rating": { "type": "number", "format": "double" },
          "price_from": { "type": "number", "format": "double" },
          "currency": { "type": "string" },
          "source": { "type": "string" },
          "external_id": { "type": "string" },
          "created_at": { "type": "string", "format": "date-time" },
          "updated_at": { "type": "string", "format": "date-time" }
        }
      },
      "Tour": {
        "type": "object",
        "required": ["destination_id", "title", "price", "duration_days"],
        "properties": {
          "id": { "type": "string", "format": "uuid" },
          "destination_id": { "type": "string", "format": "uuid" },
          "title": { "type": "string" },
          "description": { "type": "string" },
          "price": { "type": "number", "format": "double" },
          "currency": { "type": "string" },
          "duration_days": { "type": "integer" },
          "start_date": { "type": "string", "format": "date-time" },
          "end_date": { "type": "string", "format": "date-time" },
          "available": { "type": "boolean" },
          "image_url": { "type": "string" },
          "source": { "type": "string" },
          "external_id": { "type": "string" },
          "created_at": { "type": "string", "format": "date-time" },
          "updated_at": { "type": "string", "format": "date-time" }
        }
      },
      "TravelEvent": {
        "type": "object",
        "properties": {
          "id": { "type": "string", "format": "uuid" },
          "type": { "type": "string" },
          "aggregate": { "type": "string" },
          "aggregate_id": { "type": "string", "format": "uuid" },
          "payload": { "type": "string" },
          "created_at": { "type": "string", "format": "date-time" }
        }
      },
      "SyncJob": {
        "type": "object",
        "properties": {
          "id": { "type": "string", "format": "uuid" },
          "source": { "type": "string" },
          "status": { "type": "string" },
          "records": { "type": "integer" },
          "error": { "type": "string" },
          "created_at": { "type": "string", "format": "date-time" },
          "updated_at": { "type": "string", "format": "date-time" }
        }
      },
      "PaginationMeta": {
        "type": "object",
        "properties": {
          "page": { "type": "integer" },
          "page_size": { "type": "integer" },
          "total": { "type": "integer" },
          "total_pages": { "type": "integer" }
        }
      }
    }
  }
}`

type goTravelSwagger struct{}

func (s *goTravelSwagger) GetInstanceName() string {
	return "go-travel"
}

func (s *goTravelSwagger) ReadDoc() string {
	return swaggerSpec
}

func init() {
	swag.Register("go-travel", &goTravelSwagger{})
}
