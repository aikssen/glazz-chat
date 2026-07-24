import type { components } from "@glazz/contracts";

type Schemas = components["schemas"];

export type Locale = "es" | "en";
export type Theme = "light" | "dark" | "system";
export type CurrentUser = Schemas["CurrentUser"];
export type GuestAllowance = Schemas["GuestAllowance"];
export type Model = Schemas["Model"];
export type Conversation = Schemas["Conversation"];
export type Message = Schemas["Message"] & { generationId?: string };
export type Usage = Schemas["Usage"];
export type RuntimeSetting = Schemas["RuntimeSetting"];
export type AdminModel = Schemas["AdminModel"];
export type AdminUser = Schemas["AdminUser"];
