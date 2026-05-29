import {
  Mail,
  MessageSquareText,
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
    label: "DMs",
    description: "Private conversations",
    icon: Mail,
  },
];
