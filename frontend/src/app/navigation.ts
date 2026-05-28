import {
  Mail,
  MessageSquareText,
  Repeat2,
  Settings,
  UserCircle,
} from "lucide-react";

export type NavItem = {
  href: string;
  label: string;
  description: string;
  icon: typeof MessageSquareText;
};

export const navItems: NavItem[] = [
  {
    href: "#/",
    label: "Timeline",
    description: "Posts and recent activity",
    icon: MessageSquareText,
  },
  {
    href: "#/profile",
    label: "My profile",
    description: "Profile, follows, bookmarks, and favourites",
    icon: UserCircle,
  },
  {
    href: "#/direct",
    label: "Direct",
    description: "Private conversations",
    icon: Mail,
  },
  {
    href: "#/delivery",
    label: "Delivery",
    description: "Remote delivery attempts and retries",
    icon: Repeat2,
  },
  {
    href: "#/settings",
    label: "Settings",
    description: "Instance and account preferences",
    icon: Settings,
  },
];
