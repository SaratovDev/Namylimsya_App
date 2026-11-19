export type ApplicationStatus = "pending" | "approved" | "rejected" | "cancelled" | "";

export type Event = {
  id: string;
  organizer_id: string;
  title: string;
  category: string;
  description: string | null;
  starts_at: string;
  city: string;
  address: string | null;
  lat: number;
  lng: number;
  location: string;
  hide_exact_location: boolean;
  participant_limit: number | null;
  status: "active" | "cancelled" | "completed";
  cancel_reason: string | null;
  created_at: string;
  updated_at: string;
  organizer_name: string;
  organizer_avatar: string | null;
  participants_count: number;
  user_application_status: ApplicationStatus;
  user_application_id: string | null;
  pending_applications_count: number;
  can_see_exact_location: boolean;
};

export type ApplicationWithEvent = {
  id: string;
  event_id: string;
  user_id: string;
  status: "pending" | "approved" | "rejected" | "cancelled";
  cancel_reason: string | null;
  created_at: string;
  updated_at: string;
  event_title: string;
  event_starts_at: string;
  event_city: string;
  event_category: string;
  event_status: "active" | "cancelled" | "completed";
  event_cancel_reason: string | null;
  event_organizer_id: string;
  organizer_name: string;
  organizer_avatar: string | null;
};
// ─── Admin ────────────────────────────────────────────────────────────

export type AdminUserListItem = {
  id: string;
  email: string;
  username: string;
  role: "user" | "moderator" | "admin";
  is_banned: boolean;
  created_at: string;
};

export type AdminUserDetails = {
  user: {
    id: string;
    email: string;
    username: string;
    gender: string | null;
    birth_date: string | null;  // pgtype.Date → "YYYY-MM-DD" или null
    show_age: boolean;
    city: string | null;
    bio: string | null;
    hobbies: string[] | null;
    telegram: string | null;
    vk: string | null;
    avatar_url: string | null;
    role: "user" | "moderator" | "admin";
    is_banned: boolean;
    ban_reason: string | null;
    ban_expires_at: string | null;
    email_verified: boolean;
    consent_at: string | null;
    consent_version: string | null;
    age_confirmed_at: string | null;
    created_at: string;
    updated_at: string;
  };
  stats: {
    organized_events: number;
    applications_total: number;
    applications_approved: number;
    applications_pending: number;
    applications_rejected: number;
    applications_cancelled: number;
  };
};

// ─── Admin: Events ────────────────────────────────────────────────────

export type AdminEventListItem = {
  participant_limit: number | null;
  id: string;
  title: string;
  city: string;
  category: string;
  starts_at: string;
  status: "active" | "cancelled" | "completed";
  organizer_id: string;
  organizer_name: string;
  hidden: boolean;
  created_at: string;
  participants_count: number;
  pending_count: number;
};

export type AdminEventApplication = {
  id: string;
  event_id: string;
  user_id: string;
  status: "pending" | "approved" | "rejected" | "cancelled";
  cancel_reason: string | null;
  created_at: string;
  updated_at: string;
  username: string;
  avatar_url: string | null;
};

export type AdminEventMessage = {
  id: string;
  event_id: string;
  user_id: string;
  text: string;
  reply_to_id: string | null;
  created_at: string;
  deleted_at: string | null;
  user_name: string;
  user_avatar: string | null;
};

export type AdminEventDetails = {
  event: {
    id: string;
    organizer_id: string;
    title: string;
    category: string;
    description: string | null;
    starts_at: string;
    city: string;
    address: string | null;
    lat: number;
    lng: number;
    hide_exact_location: boolean;
    participant_limit: number | null;
    status: "active" | "cancelled" | "completed";
    cancel_reason: string | null;
    hidden: boolean;
    created_at: string;
    updated_at: string;
    organizer_name: string;
    organizer_email: string;
    organizer_avatar: string | null;
    participants_count: number;
    pending_count: number;
    rejected_count: number;
    messages_count: number;
  };
  applications: AdminEventApplication[] | null;
  messages: AdminEventMessage[] | null;
};

// ─── Admin: Complaints ────────────────────────────────────────────────

export type AdminComplaintStatus = "open" | "resolved" | "rejected";

export type AdminComplaintListItem = {
  id: string;
  reporter_id: string;
  target_type: "event" | "user" | "message";
  target_id: string;
  reason: string;
  status: AdminComplaintStatus;
  created_at: string;
  reporter_email: string | null;
};

export type AdminComplaintDetails = {
  complaint: AdminComplaintListItem & {
    resolution_comment: string | null;
    resolved_at: string | null;
    resolved_by: string | null;
  };
  event?: {
    id: string;
    title: string;
    status: "active" | "cancelled" | "completed";
    city: string;
    starts_at: string;
    organizer_id: string;
    organizer_name: string;
  };
  user?: {
    id: string;
    username: string;
    email: string;
    is_banned: boolean;
    role: "user" | "moderator" | "admin";
    created_at: string;
  };
  message?: {
    id: string;
    event_id: string;
    user_id: string;
    text: string;
    created_at: string;
    deleted_at: string | null;
    user_name: string;
    event_title: string;
  };
};

// ─── Admin: Support Tickets ───────────────────────────────────────────

export type AdminTicketStatus = "open" | "resolved" | "rejected";
export type AdminTicketType = "complaint" | "suggestion" | "appeal";

export type AdminTicketListItem = {
  id: string;
  user_id: string | null;
  type: AdminTicketType;
  subject: string;
  message: string;
  status: AdminTicketStatus;
  complaint_id: string | null;
  resolution_comment: string | null;
  resolved_at: string | null;
  resolved_by: string | null;
  created_at: string;
  user_email: string | null;
};

export type AdminTicketReply = {
  id: string;
  ticket_id: string;
  admin_id: string | null;
  admin_email: string;
  body: string;
  created_at: string;
};

export type AdminTicketDetails = {
  ticket: AdminTicketListItem & {
    user_username: string | null;
  };
  replies: AdminTicketReply[];
};

// ─── Admin: Audit Log ─────────────────────────────────────────────────

export type AdminAuditLogEntry = {
  id: string;
  admin_id: string | null;
  admin_email: string;
  action: string;
  target_type: string;
  target_id: string | null;
  details: Record<string, unknown> | null;
  created_at: string;
};

// ─── Public Profile ───────────────────────────────────────────────────

export type ProfileEvent = {
  id: string;
  title: string;
  category: string;
  starts_at: string;
  city: string;
  // Только в organized_events:
  participants_count?: number;
  // Только в attended_events:
  organizer_name?: string;
  organizer_avatar?: string | null;
};

export type PublicProfile = {
  id: string;
  username: string;
  gender: string | null;
  city: string | null;
  bio: string | null;
  hobbies: string[] | null;
  avatar_url: string | null;
  role: "user" | "moderator" | "admin";
  is_banned: boolean;
  age?: number | null;
  created_at: string;
  days_on_platform: number;
  organized_count: number;
  attended_count: number;
  title: string;
  organized_events: ProfileEvent[];
  attended_events: ProfileEvent[];
};

// ─── Username history ─────────────────────────────────────────────────

export type UsernameHistoryEntry = {
  id: string;
  old_username: string;
  changed_at: string;
};