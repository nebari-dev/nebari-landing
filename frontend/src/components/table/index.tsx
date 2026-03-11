import type{
  ReactNode
} from "react";

import {
  Table, Tag, Button
} from "@trussworks/react-uswds";

import {
  StatusTag
} from "../statustag"

import
  pinIconUrl
from "../../assets/pin.svg";

import
  unpinIconUrl
from "../../assets/unpin.svg";

import AvatarImage from "../icon"

import {
  cn
} from '../../lib/cn'

import "./index.scss";

export default function AppTable(props: AppTableProps): ReactNode {
  
  const {
    className,
    rows,
    responsive = "stacked-header",
    bordered = true,
    striped = false,
    compact = false,

    onTogglePin,
  } = props

  const stackedStyle =
    responsive === "stacked-header"
      ? ("headers" as const)
      : responsive === "stacked"
        ? ("default" as const)
        : ("none" as const);

  const scrollable = responsive === "scrollable";

  const handleTogglePin =
    onTogglePin ??
    ((rowId: string, nextPinned: boolean) =>
      alert(`${nextPinned ? "Pinned" : "Unpinned"}: ${rowId}`));

  return (
    <div className={cn("app-table", className)}>
      <Table
        bordered={bordered}
        striped={striped}
        compact={compact}
        fullWidth
        scrollable={scrollable}
        stackedStyle={stackedStyle}
      >
        <thead>
          <tr>
            <th scope="col">Service</th>
            <th scope="col">Category</th>
            <th scope="col">Status</th>
            <th scope="col" className="app-table__actionHeader">
              Action
            </th>
          </tr>
        </thead>

        <tbody>
          {rows.map((row) => {

            const title = row.title;
            const description = row.description;
            const status = row.status;
            const categories = row.categories;
            const pinned = !!row.pinned;
            const url = row.url;
            const image = row.image;

            return (
              <tr key={row.id}>
                <th
                  scope="row"
                  data-label="Service"
                  className="app-table__serviceCell"
                >
                  <a
                    href={url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="app-table__serviceLink"
                    aria-label={`${title} (opens in a new tab)`}
                  >
                    <div className="app-table__service">
                      <div className="app-table__avatar">
                        {AvatarImage(image)}
                      </div>

                      <div className="app-table__serviceMeta">
                        <div className="app-table__serviceTitle">{title}</div>
                        <div className="app-table__serviceDescription">
                          {description}
                        </div>
                      </div>
                    </div>
                  </a>
                </th>

                {/* TODO: Split thhis into a separate component */}
                <td data-label="Category">
                  <div className="app-table__categories">
                    {categories.map((item) => (
                      <Tag
                        key={item}
                        background="base-lighter"
                        className="app-table__categoryItem"
                      >
                        {item}
                      </Tag>
                    ))}
                  </div>
                </td>

                <td data-label="Status">
                  <StatusTag status={status} />
                </td>

                <td data-label="Action" className="app-table__actionCell">
                  <Button
                    type="button"
                    unstyled
                    className="app-table__pinButton"
                    onClick={() => handleTogglePin(row.id, !pinned)}
                    title={pinned ? "Unpin" : "Pin"}
                  >
                    <img
                      src={pinned ? unpinIconUrl : pinIconUrl}
                      className="app-table__pinIcon"
                    />
                  </Button>
                </td>
              </tr>
            );
          })}
        </tbody>
      </Table>
    </div>
  );
}

export type AppTableRow = {
  id: string;
  title: string;
  description: string;
  image: string;
  imageAlt?: string;
  categories: string[];
  status: string;
  url: string
  pinned: boolean;
};

export type AppTableProps = {
  className?: string;
  caption?: ReactNode;
  rows: AppTableRow[];
  responsive?: "stacked-header" | "stacked" | "scrollable" | "none";
  bordered?: boolean;
  striped?: boolean;
  compact?: boolean;
  onTogglePin?: (rowId: string, nextPinned: boolean) => void;
};
