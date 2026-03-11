import type { ReactNode } from "react";
import type { Notification } from "../../api/notifications";

import { Collection } from "@trussworks/react-uswds";

import AvatarImage from "../icon";

import "./notificationsList.scss";

type NotificationListProps = {
  notifications: Notification[];
};

export default function NotificationList(
  props: NotificationListProps
): ReactNode {
  const { notifications } = props;

  if (notifications.length === 0) {
    return (
      <div className="app-notificationList__empty" role="status">
        No notifications.
      </div>
    );
  }

  return (
    <Collection className="app-notificationList usa-collection--condensed margin-y-0">
      {notifications.map((notification) => (
        <li
          key={notification.id}
          className={`usa-collection__item app-notificationList__item${
            notification.read ? " app-notificationList__item--read" : ""
          }`}
        >
          <div className="app-notificationList__image" aria-hidden="true">
            {AvatarImage(notification.image)}
          </div>

          <div className="usa-collection__body">
            <h4 className="usa-collection__heading app-notificationList__title">
              {notification.title}
            </h4>

            <p className="usa-collection__description app-notificationList__description">
              {notification.message}
            </p>
          </div>
        </li>
      ))}
    </Collection>
  );
}
